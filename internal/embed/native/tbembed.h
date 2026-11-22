/*
 * tbembed.h: the only C interface between tb and llama.cpp (ADR-018).
 * Every call takes caller-owned buffers with explicit sizes; nothing here
 * reads files other than the model, opens sockets or keeps global state
 * beyond llama.cpp's own backend.
 */
#ifndef TB_EMBED_H
#define TB_EMBED_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

enum tbe_status {
	TBE_OK = 0,
	TBE_ARGS = 1,	     /* invalid argument */
	TBE_MODEL_LOAD = 2,  /* the model file did not load */
	TBE_UNSUPPORTED = 3, /* pooling NONE or RANK: no sentence vector */
	TBE_TOKENIZE = 4,    /* tokenization failed */
	TBE_EVAL = 5,	     /* encode/decode failed */
	TBE_NOMEM = 6	     /* allocation failed */
};

typedef struct tbe_model tbe_model;

/* tbe_open loads the GGUF model at path (CPU only, no mmap of anything
 * else) and prepares an embedding context. On failure *out is NULL. */
int tbe_open(const char *path, int32_t threads, tbe_model **out);

/* tbe_dims is the vector length; tbe_n_ctx the token limit per text. */
int32_t tbe_dims(const tbe_model *m);
int32_t tbe_n_ctx(const tbe_model *m);

/* tbe_embed writes one L2-normalized vector of tbe_dims floats for the
 * text (len bytes, not NUL-terminated) to out, which holds out_len
 * floats. *truncated is 1 when the text was cut to tbe_n_ctx tokens. */
int tbe_embed(tbe_model *m, const char *text, size_t len, float *out, size_t out_len, int *truncated);

/* TBE_MAX_SEQ is the most texts one tbe_embed_batch call takes. */
#define TBE_MAX_SEQ 64

/* tbe_embed_batch embeds n texts (1..TBE_MAX_SEQ; texts[i] holds lens[i]
 * bytes) into out, n rows of tbe_dims floats (out_len floats in all), and
 * sets truncated[i] like tbe_embed. Texts are packed into as few
 * evaluations as the token budget allows; on error no row is valid. */
int tbe_embed_batch(tbe_model *m, const char *const *texts, const size_t *lens, int32_t n, float *out,
		    size_t out_len, int *truncated);

void tbe_close(tbe_model *m);

#ifdef __cplusplus
}
#endif

#endif /* TB_EMBED_H */
