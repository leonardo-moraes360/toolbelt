//go:build embednative && cgo

/*
 * tbembed.c: the shim over llama.cpp's C API (ADR-018). It embeds one text
 * at a time as sequence 0, clearing the context memory between texts.
 */
#include "tbembed.h"

#include <limits.h>
#include <math.h>
#include <stdlib.h>
#include <string.h>

#include "llama.h"

/* Upper bound on the context, whatever the model was trained with. */
#define TBE_MAX_CTX 8192

struct tbe_model {
	struct llama_model *model;
	struct llama_context *ctx;
	const struct llama_vocab *vocab;
	int32_t dims;
	int32_t n_ctx;
	int encoder_only;
	llama_token *tokens; /* n_ctx tokens */
};

static void tbe_quiet_log(enum ggml_log_level level, const char *text, void *user_data)
{
	(void)level;
	(void)text;
	(void)user_data;
}

static void tbe_free(tbe_model *m)
{
	if (m == NULL) {
		return;
	}
	if (m->ctx != NULL) {
		llama_free(m->ctx);
	}
	if (m->model != NULL) {
		llama_model_free(m->model);
	}
	free(m->tokens);
	free(m);
}

static int tbe_context(tbe_model *m, int32_t threads)
{
	struct llama_context_params cp = llama_context_default_params();
	int32_t train = llama_model_n_ctx_train(m->model);
	uint32_t n_ctx = (train > 0 && train < TBE_MAX_CTX) ? (uint32_t)train : TBE_MAX_CTX;
	enum llama_pooling_type pooling;

	cp.embeddings = true;
	cp.n_ctx = n_ctx;
	cp.n_batch = n_ctx;
	cp.n_ubatch = n_ctx; /* non-causal models need the whole text in one ubatch */
	cp.n_seq_max = 1;
	cp.n_threads = threads;
	cp.n_threads_batch = threads;
	m->ctx = llama_init_from_model(m->model, cp);
	if (m->ctx == NULL) {
		return TBE_MODEL_LOAD;
	}
	pooling = llama_pooling_type(m->ctx);
	if (pooling == LLAMA_POOLING_TYPE_NONE || pooling == LLAMA_POOLING_TYPE_RANK) {
		return TBE_UNSUPPORTED;
	}
	m->n_ctx = (int32_t)llama_n_ctx(m->ctx);
	return TBE_OK;
}

int tbe_open(const char *path, int32_t threads, tbe_model **out)
{
	struct llama_model_params mp;
	tbe_model *m;
	int rc;

	if (out == NULL) {
		return TBE_ARGS;
	}
	*out = NULL;
	if (path == NULL || threads < 1 || threads > 256) {
		return TBE_ARGS;
	}
	llama_log_set(tbe_quiet_log, NULL);
	llama_backend_init();
	m = calloc(1, sizeof(*m));
	if (m == NULL) {
		return TBE_NOMEM;
	}
	mp = llama_model_default_params();
	mp.n_gpu_layers = 0;
	/* Read-only mmap; never mlock, never direct I/O. */
	mp.load_mode = LLAMA_LOAD_MODE_MMAP;
	m->model = llama_model_load_from_file(path, mp);
	if (m->model == NULL) {
		tbe_free(m);
		return TBE_MODEL_LOAD;
	}
	m->vocab = llama_model_get_vocab(m->model);
	/* The pooled sequence vector has n_embd_out floats, not n_embd. */
	m->dims = llama_model_n_embd_out(m->model);
	m->encoder_only = llama_model_has_encoder(m->model) && !llama_model_has_decoder(m->model);
	rc = tbe_context(m, threads);
	if (rc == TBE_OK && (m->dims < 1 || m->n_ctx < 1)) {
		rc = TBE_UNSUPPORTED;
	}
	if (rc == TBE_OK) {
		m->tokens = calloc((size_t)m->n_ctx, sizeof(*m->tokens));
		if (m->tokens == NULL) {
			rc = TBE_NOMEM;
		}
	}
	if (rc != TBE_OK) {
		tbe_free(m);
		return rc;
	}
	*out = m;
	return TBE_OK;
}

int32_t tbe_dims(const tbe_model *m)
{
	return m == NULL ? 0 : m->dims;
}

int32_t tbe_n_ctx(const tbe_model *m)
{
	return m == NULL ? 0 : m->n_ctx;
}

/* tbe_tokenize fills m->tokens; it returns the count, or -1 on failure. */
static int32_t tbe_tokenize(tbe_model *m, const char *text, int32_t len, int *truncated)
{
	int32_t n = llama_tokenize(m->vocab, text, len, m->tokens, m->n_ctx, true, false);

	*truncated = 0;
	if (n == INT32_MIN) {
		return -1;
	}
	if (n < 0) {
		/* Too many tokens: keep the first n_ctx - 1 and the last one, which
		 * is the trailing special token (SEP/EOS) when the vocab adds one. */
		int32_t need = -n;
		llama_token *all = calloc((size_t)need, sizeof(*all));
		if (all == NULL) {
			return -1;
		}
		n = llama_tokenize(m->vocab, text, len, all, need, true, false);
		if (n < 0) {
			free(all);
			return -1;
		}
		memcpy(m->tokens, all, (size_t)(m->n_ctx - 1) * sizeof(*all));
		m->tokens[m->n_ctx - 1] = all[n - 1];
		free(all);
		n = m->n_ctx;
		*truncated = 1;
	}
	return n;
}

static int tbe_eval(tbe_model *m, int32_t n)
{
	struct llama_batch_ext *batch = llama_batch_ext_init(m->ctx);
	int rc = TBE_OK;

	if (batch == NULL) {
		return TBE_NOMEM;
	}
	for (int32_t i = 0; i < n && rc == TBE_OK; i++) {
		llama_pos pos = i;
		int32_t idx = llama_batch_ext_add_token(batch, 0, m->tokens[i]);

		if (idx < 0 || !llama_batch_ext_set_pos(batch, idx, &pos) ||
		    !llama_batch_ext_set_output_embd(batch, idx, true)) {
			rc = TBE_EVAL;
		}
	}
	if (rc == TBE_OK) {
		llama_memory_clear(llama_get_memory(m->ctx), true);
		if (llama_process(m->ctx,
				  m->encoder_only ? LLAMA_PROCESS_TYPE_ENCODE : LLAMA_PROCESS_TYPE_DECODE,
				  batch) != 0) {
			rc = TBE_EVAL;
		}
	}
	llama_batch_ext_free(batch);
	return rc;
}

int tbe_embed(tbe_model *m, const char *text, size_t len, float *out, size_t out_len, int *truncated)
{
	const float *v;
	double norm = 0.0;
	int32_t n;
	int rc;

	if (m == NULL || text == NULL || out == NULL || truncated == NULL || len > (size_t)INT32_MAX ||
	    out_len < (size_t)m->dims) {
		return TBE_ARGS;
	}
	n = tbe_tokenize(m, text, (int32_t)len, truncated);
	if (n < 0) {
		return TBE_TOKENIZE;
	}
	if (n == 0) {
		return TBE_TOKENIZE;
	}
	rc = tbe_eval(m, n);
	if (rc != TBE_OK) {
		return rc;
	}
	v = llama_get_embeddings_seq(m->ctx, 0);
	if (v == NULL) {
		return TBE_EVAL;
	}
	for (int32_t i = 0; i < m->dims; i++) {
		norm += (double)v[i] * (double)v[i];
	}
	norm = sqrt(norm);
	for (int32_t i = 0; i < m->dims; i++) {
		out[i] = norm > 0.0 ? (float)((double)v[i] / norm) : 0.0f;
	}
	return TBE_OK;
}

void tbe_close(tbe_model *m)
{
	tbe_free(m);
}
