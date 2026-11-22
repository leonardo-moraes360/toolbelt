//go:build embednative && cgo

/*
 * tbembed.c: the shim over llama.cpp's C API (ADR-018). A batch call packs
 * up to TBE_MAX_SEQ texts, one sequence each, into as few evaluations as
 * the token budget (n_ubatch) allows, clearing the context memory between
 * evaluations.
 */
#include "tbembed.h"

#include <limits.h>
#include <math.h>
#include <stdlib.h>
#include <string.h>

#include "llama.h"

/* Upper bound on the context, whatever the model was trained with. */
#define TBE_MAX_CTX 8192
/* Packing cap of one evaluation. A non-causal model reads the whole batch
 * in one ubatch and its attention is computed densely over every packed
 * token (other sequences are only masked), so packing past this costs more
 * attention than it saves in calls. A single longer text still runs alone,
 * up to n_ctx. */
#define TBE_BATCH_TOKENS 512

struct tbe_model {
	struct llama_model *model;
	struct llama_context *ctx;
	const struct llama_vocab *vocab;
	int32_t dims;
	int32_t n_ctx;
	int encoder_only;
	int32_t budget;	     /* packing cap: min(n_ubatch, TBE_BATCH_TOKENS) */
	llama_token *tokens; /* TBE_MAX_SEQ rows of n_ctx tokens */
	int32_t *n_tokens;   /* per row */
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
	free(m->n_tokens);
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
	/* Non-causal models need every packed text in one ubatch; llama.cpp
	 * caps both at n_ctx for causal ones. */
	cp.n_batch = n_ctx > TBE_BATCH_TOKENS ? n_ctx : TBE_BATCH_TOKENS;
	cp.n_ubatch = cp.n_batch;
	cp.n_seq_max = TBE_MAX_SEQ;
	/* One shared buffer: each sequence keeps the full n_ctx instead of
	 * n_ctx / n_seq_max. */
	cp.kv_unified = true;
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
	if (m->n_ctx > (int32_t)n_ctx) {
		/* The context is padded; the per-text limit stays the requested one. */
		m->n_ctx = (int32_t)n_ctx;
	}
	{
		int32_t ubatch = (int32_t)llama_n_ubatch(m->ctx);

		if (ubatch < m->n_ctx) {
			m->n_ctx = ubatch;
		}
		m->budget = ubatch < TBE_BATCH_TOKENS ? ubatch : TBE_BATCH_TOKENS;
	}
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
		m->tokens = calloc((size_t)TBE_MAX_SEQ * (size_t)m->n_ctx, sizeof(*m->tokens));
		m->n_tokens = calloc(TBE_MAX_SEQ, sizeof(*m->n_tokens));
		if (m->tokens == NULL || m->n_tokens == NULL) {
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

/* tbe_tokenize fills row (n_ctx tokens); it returns the count, or -1 on
 * failure. */
static int32_t tbe_tokenize(tbe_model *m, llama_token *row, const char *text, int32_t len, int *truncated)
{
	int32_t n = llama_tokenize(m->vocab, text, len, row, m->n_ctx, true, false);

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
		memcpy(row, all, (size_t)(m->n_ctx - 1) * sizeof(*all));
		row[m->n_ctx - 1] = all[n - 1];
		free(all);
		n = m->n_ctx;
		*truncated = 1;
	}
	return n;
}

/* tbe_eval evaluates rows [first, first+count) as sequences 0..count-1. */
static int tbe_eval(tbe_model *m, int32_t first, int32_t count)
{
	struct llama_batch_ext *batch = llama_batch_ext_init(m->ctx);
	int rc = TBE_OK;

	if (batch == NULL) {
		return TBE_NOMEM;
	}
	for (int32_t s = 0; s < count && rc == TBE_OK; s++) {
		const llama_token *row = m->tokens + (size_t)(first + s) * (size_t)m->n_ctx;

		for (int32_t i = 0; i < m->n_tokens[first + s] && rc == TBE_OK; i++) {
			llama_pos pos = i;
			int32_t idx = llama_batch_ext_add_token(batch, s, row[i]);

			if (idx < 0 || !llama_batch_ext_set_pos(batch, idx, &pos) ||
			    !llama_batch_ext_set_output_embd(batch, idx, true)) {
				rc = TBE_EVAL;
			}
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

/* tbe_pooled copies sequence seq's vector, L2-normalized, to out. */
static int tbe_pooled(tbe_model *m, int32_t seq, float *out)
{
	const float *v = llama_get_embeddings_seq(m->ctx, seq);
	double norm = 0.0;

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

int tbe_embed_batch(tbe_model *m, const char *const *texts, const size_t *lens, int32_t n, float *out,
		    size_t out_len, int *truncated)
{
	int32_t first = 0;

	if (m == NULL || texts == NULL || lens == NULL || out == NULL || truncated == NULL || n < 1 ||
	    n > TBE_MAX_SEQ || out_len / (size_t)m->dims < (size_t)n) {
		return TBE_ARGS;
	}
	for (int32_t i = 0; i < n; i++) {
		llama_token *row = m->tokens + (size_t)i * (size_t)m->n_ctx;

		if (texts[i] == NULL || lens[i] > (size_t)INT32_MAX) {
			return TBE_ARGS;
		}
		m->n_tokens[i] = tbe_tokenize(m, row, texts[i], (int32_t)lens[i], &truncated[i]);
		if (m->n_tokens[i] <= 0) {
			return TBE_TOKENIZE;
		}
	}
	/* Greedy packing in order. The first row of a group is always taken,
	 * even past the cap (it fits n_ubatch, since n_ctx <= n_ubatch), so every
	 * pass makes progress. */
	while (first < n) {
		int32_t count = 1;
		int32_t used = m->n_tokens[first];
		int rc;

		while (first + count < n && used + m->n_tokens[first + count] <= m->budget) {
			used += m->n_tokens[first + count];
			count++;
		}
		rc = tbe_eval(m, first, count);
		for (int32_t s = 0; s < count && rc == TBE_OK; s++) {
			rc = tbe_pooled(m, s, out + (size_t)(first + s) * (size_t)m->dims);
		}
		if (rc != TBE_OK) {
			return rc;
		}
		first += count;
	}
	return TBE_OK;
}

int tbe_embed(tbe_model *m, const char *text, size_t len, float *out, size_t out_len, int *truncated)
{
	return tbe_embed_batch(m, &text, &len, 1, out, out_len, truncated);
}

void tbe_close(tbe_model *m)
{
	tbe_free(m);
}
