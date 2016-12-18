/*
 * Copyright (c) 2016 Alex Yatskov <alex@foosoft.net>
 * Author: Alex Yatskov <alex@foosoft.net>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to
 * use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
 * the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
 * IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

package main

import (
	"io"
	"strings"

	"github.com/FooSoft/jmdict"
)

func computeJmdictRules(term *dbTerm) {
	for _, tag := range term.Tags {
		switch tag {
		case "adj-i", "v1", "vk", "vs":
			term.addRules(tag)
		default:
			if strings.HasPrefix(tag, "v5") {
				term.addRules("v5")
			}
		}
	}
}

func computeJmdictScore(term *dbTerm) {
	term.Score = 0
	for _, tag := range term.Tags {
		switch tag {
		case "gai1", "ichi1", "news1", "spec1":
			term.Score += 5
		case "arch", "iK":
			term.Score -= 1
		}
	}
}

func computeJmdictTagMeta(entities map[string]string) map[string]dbTagMeta {
	tags := map[string]dbTagMeta{
		"news1": {Notes: "appears frequently in Mainichi Shimbun (top listing)", Category: "frequent", Order: 3},
		"ichi1": {Notes: "listed as common in Ichimango Goi Bunruishuu (top listing)", Category: "frequent", Order: 3},
		"spec1": {Notes: "common words not included in frequency lists (top listing)", Category: "frequent", Order: 3},
		"gai1":  {Notes: "common loanword (top listing)", Category: "frequent", Order: 3},
		"news2": {Notes: "appears frequently in Mainichi Shimbun (bottom listing)", Order: 3},
		"ichi2": {Notes: "listed as common in Ichimango Goi Bunruishuu (bottom listing)", Order: 3},
		"spec2": {Notes: "common words not included in frequency lists (bottom listing)", Order: 3},
		"gai2":  {Notes: "common loanword (bottom listing)", Order: 3},
	}

	for name, value := range entities {
		tag := dbTagMeta{Notes: value}

		switch name {
		case "gai1", "ichi1", "news1", "spec1":
			tag.Category = "frequent"
			tag.Order = 1
		case "exp", "id":
			tag.Category = "expression"
			tag.Order = 2
		case "arch", "iK":
			tag.Category = "archaism"
			tag.Order = 2
		}

		tags[name] = tag
	}

	return tags
}

func extractJmdictTerms(edictEntry jmdict.JmdictEntry) []dbTerm {
	var terms []dbTerm

	convert := func(reading jmdict.JmdictReading, kanji *jmdict.JmdictKanji) {
		if kanji != nil && reading.Restrictions != nil && !hasString(kanji.Expression, reading.Restrictions) {
			return
		}

		var termBase dbTerm
		termBase.addTags(reading.Information...)

		if kanji == nil {
			termBase.Expression = reading.Reading
			termBase.addTags(reading.Priorities...)
		} else {
			termBase.Expression = kanji.Expression
			termBase.Reading = reading.Reading
			termBase.addTags(kanji.Information...)

			for _, priority := range kanji.Priorities {
				if hasString(priority, reading.Priorities) {
					termBase.addTags(priority)
				}
			}
		}

		for _, sense := range edictEntry.Sense {
			if sense.RestrictedReadings != nil && !hasString(reading.Reading, sense.RestrictedReadings) {
				continue
			}

			if kanji != nil && sense.RestrictedKanji != nil && !hasString(kanji.Expression, sense.RestrictedKanji) {
				continue
			}

			term := dbTerm{Reading: termBase.Reading, Expression: termBase.Expression}
			term.addTags(termBase.Tags...)
			term.addTags(sense.PartsOfSpeech...)
			term.addTags(sense.Fields...)
			term.addTags(sense.Misc...)
			term.addTags(sense.Dialects...)

			for _, glossary := range sense.Glossary {
				term.Glossary = append(term.Glossary, glossary.Content)
			}

			computeJmdictRules(&term)
			computeJmdictScore(&term)

			terms = append(terms, term)
		}
	}

	if len(edictEntry.Kanji) > 0 {
		for _, kanji := range edictEntry.Kanji {
			for _, reading := range edictEntry.Readings {
				convert(reading, &kanji)
			}
		}
	} else {
		for _, reading := range edictEntry.Readings {
			convert(reading, nil)
		}
	}

	return terms
}

func exportJmdictDb(outputDir, title string, reader io.Reader, flags int) error {
	dict, entities, err := jmdict.LoadJmdictNoTransform(reader)
	if err != nil {
		return err
	}

	var terms dbTermList
	for _, entry := range dict.Entries {
		terms = append(terms, extractJmdictTerms(entry)...)
	}

	return writeDb(
		outputDir,
		title,
		terms.crush(),
		nil,
		computeJmdictTagMeta(entities),
		flags&flagPretty == flagPretty,
	)
}
