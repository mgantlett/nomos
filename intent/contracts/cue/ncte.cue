package schemas

import "strings"

// #NCTEBounds declares numeric constraints for Markdown prose under SPEC-NCTE-100.
#NCTEBounds: close({
	max_descriptive_words:   (int & <=25) | *25
	max_procedural_words:    (int & <=20) | *20
	max_paragraph_sentences: (int & <=6) | *6
	max_table_cell_words:    (int & <=12) | *12
	max_passive_adverb_gap:  (int & <=2) | *2
})

// #VocabularyDefinition declares a canonical term in the NCTE-100 Succinct Action Lexicon.
#VocabularyDefinition: close({
	domain:           "lifecycle" | "verification" | "isolation" | "memory" | "steering"
	canonical_term:   string & strings.MinRunes(1)
	part_of_speech:   "verb" | "noun" | "adjective"
	approved_meaning: string & strings.MinRunes(1)
	banned_synonyms:  [...string & strings.MinRunes(1)]
	cli_command?:     string
})

// #NCTEPolicy defines the authoritative validation contract for Nomos Controlled Technical English.
#NCTEPolicy: close({
	bounds: #NCTEBounds
	banned_modals: [...string] | *[
		"should",
		"would",
		"could",
		"might",
		"ideally",
		"as needed",
		"and/or",
	]
	exempt_table_headers: [...string] | *[
		"Input Sentence",
		"Approved Semantic Meaning",
		"Banned Synonyms",
	]
	severity:   "ERROR"
	vocabulary: [...#VocabularyDefinition]
})
