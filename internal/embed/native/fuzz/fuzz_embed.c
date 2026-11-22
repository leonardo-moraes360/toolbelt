//go:build ignore

/*
 * fuzz_embed.c: libFuzzer harness over the shim (ADR-018 native gate). It
 * loads the CI smoke model once (TB_EMBED_MODEL) and feeds each input to
 * tbe_embed as one text, then to tbe_embed_batch split on newlines (up to
 * TBE_MAX_SEQ texts), so the packing paths see hostile input too. Built by
 * scripts/native-fuzz.sh, never by go build.
 */
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>

#include "../tbembed.h"

static tbe_model *model;
static float *vec;

int LLVMFuzzerInitialize(int *argc, char ***argv);
int LLVMFuzzerTestOneInput(const uint8_t *data, size_t size);

int LLVMFuzzerInitialize(int *argc, char ***argv)
{
	const char *path = getenv("TB_EMBED_MODEL");

	(void)argc;
	(void)argv;
	if (path == NULL || tbe_open(path, 1, &model) != TBE_OK) {
		fprintf(stderr, "fuzz_embed: TB_EMBED_MODEL must name a loadable model\n");
		abort();
	}
	vec = calloc((size_t)TBE_MAX_SEQ * (size_t)tbe_dims(model), sizeof(*vec));
	if (vec == NULL) {
		abort();
	}
	return 0;
}

int LLVMFuzzerTestOneInput(const uint8_t *data, size_t size)
{
	const char *texts[TBE_MAX_SEQ];
	size_t lens[TBE_MAX_SEQ];
	int truncated[TBE_MAX_SEQ];
	int32_t n = 0;
	size_t start = 0;

	(void)tbe_embed(model, (const char *)data, size, vec, (size_t)tbe_dims(model), &truncated[0]);
	for (size_t i = 0; i <= size && n < TBE_MAX_SEQ; i++) {
		if (i == size || data[i] == '\n') {
			texts[n] = (const char *)data + start;
			lens[n] = i - start;
			n++;
			start = i + 1;
		}
	}
	(void)tbe_embed_batch(model, texts, lens, n, vec, (size_t)TBE_MAX_SEQ * (size_t)tbe_dims(model),
			      truncated);
	return 0;
}
