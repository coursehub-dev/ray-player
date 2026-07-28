import assert from "node:assert/strict";
import test from "node:test";

import { hasSuggestionQuery, shouldShowSuggestions } from "./searchUi.ts";

test("suggestions start after the first non-space character", () => {
	assert.equal(hasSuggestionQuery(""), false);
	assert.equal(hasSuggestionQuery("   "), false);
	assert.equal(hasSuggestionQuery("b"), true);
	assert.equal(hasSuggestionQuery(" б "), true);
});

test("suggestion panel requires focus, a query and at least one result", () => {
	assert.equal(shouldShowSuggestions({ focused: true, query: "b", count: 1 }), true);
	assert.equal(shouldShowSuggestions({ focused: false, query: "b", count: 1 }), false);
	assert.equal(shouldShowSuggestions({ focused: true, query: "", count: 1 }), false);
	assert.equal(shouldShowSuggestions({ focused: true, query: "b", count: 0 }), false);
});
