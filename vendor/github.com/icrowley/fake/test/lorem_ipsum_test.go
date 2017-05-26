package test

import (
	"testing"

	"github.com/icrowley/fake"
)

func TestLoremIpsum(t *testing.T) {
	for _, lang := range fake.GetLangs() {
		fake.SetLang(lang)

		v := fake.Character()
		if v == "" {
			t.Errorf("Character failed with lang %s", lang)
		}

		v = fake.CharactersN(2)
		if v == "" {
			t.Errorf("CharactersN failed with lang %s", lang)
		}

		v = fake.Characters()
		if v == "" {
			t.Errorf("Characters failed with lang %s", lang)
		}

		v = fake.Word()
		if v == "" {
			t.Errorf("Word failed with lang %s", lang)
		}

		v = fake.WordsN(2)
		if v == "" {
			t.Errorf("WordsN failed with lang %s", lang)
		}

		v = fake.Words()
		if v == "" {
			t.Errorf("Words failed with lang %s", lang)
		}

		v = fake.Title()
		if v == "" {
			t.Errorf("Title failed with lang %s", lang)
		}

		v = fake.Sentence()
		if v == "" {
			t.Errorf("Sentence failed with lang %s", lang)
		}

		v = fake.SentencesN(2)
		if v == "" {
			t.Errorf("SentencesN failed with lang %s", lang)
		}

		v = fake.Sentences()
		if v == "" {
			t.Errorf("Sentences failed with lang %s", lang)
		}

		v = fake.Paragraph()
		if v == "" {
			t.Errorf("Paragraph failed with lang %s", lang)
		}

		v = fake.ParagraphsN(2)
		if v == "" {
			t.Errorf("ParagraphsN failed with lang %s", lang)
		}

		v = fake.Paragraphs()
		if v == "" {
			t.Errorf("Paragraphs failed with lang %s", lang)
		}
	}
}
