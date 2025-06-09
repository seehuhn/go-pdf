// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package gibberish

import (
	"math/rand"
	"strings"
)

// Generator generates random text.
type Generator struct {
	chain map[string][]string

	// starts holds words that can start a sentence.
	starts []string

	rng *rand.Rand
}

// NewGenerator creates a new text generator with the given seed.
func NewGenerator(seed uint64) *Generator {
	generator := &Generator{
		chain:  make(map[string][]string),
		starts: make([]string, 0),
		rng:    rand.New(rand.NewSource(int64(seed))),
	}

	generator.buildChain(corpus)
	return generator
}

// buildChain analyzes the input text and builds a Markov chain for text generation.
func (g *Generator) buildChain(text string) {
	// Clean and split the text
	text = strings.ToLower(text)
	words := strings.Fields(text)

	// Build the chain
	for i := 0; i < len(words)-1; i++ {
		currentWord := words[i]
		nextWord := words[i+1]

		g.chain[currentWord] = append(g.chain[currentWord], nextWord)

		// Track words that can start sentences
		if i == 0 || strings.HasSuffix(words[i-1], ".") {
			g.starts = append(g.starts, currentWord)
		}
	}
}

// startNewSentence returns a random word that can start a sentence.
func (g *Generator) startNewSentence() string {
	return g.starts[g.rng.Intn(len(g.starts))]
}

// getNextWord returns the next word in the chain based on the current word.
func (g *Generator) getNextWord(currentWord string) string {
	nextWords, exists := g.chain[currentWord]
	if !exists || len(nextWords) == 0 {
		// If no continuation exists, start a new sentence
		return g.startNewSentence()
	}
	return nextWords[g.rng.Intn(len(nextWords))]
}

// GenerateText generates random text with the specified number of words.
func (g *Generator) GenerateText(wordCount int) string {
	if wordCount <= 0 {
		return ""
	}

	var result []string
	currentWord := g.startNewSentence()

	for i := 0; i < wordCount; i++ {
		result = append(result, currentWord)

		// Add punctuation occasionally to create natural sentence breaks
		if g.rng.Float32() < 0.12 && i > 8 && i < wordCount-5 {
			if !strings.HasSuffix(currentWord, ".") && !strings.HasSuffix(currentWord, ",") {
				result[len(result)-1] = currentWord + "."
				currentWord = g.startNewSentence()
			}
		} else {
			currentWord = g.getNextWord(currentWord)
		}
	}

	// Ensure the text ends with a period
	if len(result) > 0 {
		lastWord := result[len(result)-1]
		if !strings.HasSuffix(lastWord, ".") && !strings.HasSuffix(lastWord, "!") && !strings.HasSuffix(lastWord, "?") {
			result[len(result)-1] = lastWord + "."
		}
	}

	// Capitalize first letter and letters after periods
	text := strings.Join(result, " ")
	return g.capitalizeSentences(text)
}

// capitalizeSentences capitalizes the first letter and letters after periods.
func (g *Generator) capitalizeSentences(text string) string {
	if len(text) == 0 {
		return text
	}

	result := []rune(text)

	// Capitalize first character
	if len(result) > 0 && result[0] >= 'a' && result[0] <= 'z' {
		result[0] = result[0] - 32
	}

	// Capitalize after periods
	for i := 0; i < len(result)-2; i++ {
		if result[i] == '.' && result[i+1] == ' ' && result[i+2] >= 'a' && result[i+2] <= 'z' {
			result[i+2] = result[i+2] - 32
		}
	}

	return string(result)
}

// Generate is a convenience function that creates a throwaway Generator
// and uses it to generate text with the specified word count and seed.
func Generate(numWords int, seed uint64) string {
	generator := NewGenerator(seed)
	return generator.GenerateText(numWords)
}

const corpus = `
Sarah walked down the narrow street, her footsteps echoing against the cobblestones. The morning mist clung to the buildings like memories refusing to fade.
He opened the letter with trembling hands. The words on the page seemed to blur together, but their meaning was crystal clear.
The old bookstore smelled of dust and forgotten stories. Emma ran her fingers along the spines, searching for something she couldn't quite name.
Thunder rumbled in the distance as Marcus hurried home. The first drops of rain began to fall, turning the sidewalk dark and slick.
She laughed despite herself, the sound bright and unexpected in the quiet room. It had been so long since anything had seemed funny.
The train pulled into the station with a great hiss of steam. James checked his watch and realized he was already late for the most important meeting of his life.
Anna sat by the window, watching the seasons change outside. Each falling leaf reminded her of another day that had slipped away.
The door creaked open, and a figure stepped into the shadowy hallway. David held his breath, hoping he hadn't been discovered.
Coffee shops had always been her sanctuary. Claire ordered her usual and found a corner table where she could write undisturbed.
The mountains rose before them like ancient guardians, their peaks hidden in clouds. This was the moment they had trained for all summer.
His grandmother's house still smelled like cinnamon and vanilla. Every room held a different memory from his childhood.
The phone rang three times before she answered. Her voice sounded tired, older than he remembered.
Streets that once bustled with activity now lay empty and silent. The pandemic had changed everything, perhaps forever.
She found the photograph tucked between the pages of an old book. The faces staring back at her belonged to people she had never met.
The garden was overgrown, but beautiful in its wildness. Roses climbed the fence in tangled profusion, their petals scattered by the wind.
Robert stood at the crossroads, literally and figuratively. Each path led to a different future, and he couldn't decide which one to choose.
The concert hall filled with music so beautiful it brought tears to her eyes. This was why she had become a musician in the first place.
Children played in the park while their parents watched from nearby benches. The afternoon sun painted everything in golden light.
He discovered the hidden room behind the bookshelf by accident. Inside, boxes of letters told a story no one had ever heard.
The boat rocked gently in the harbor, its sails furled for the night. Tomorrow they would set sail for distant shores.
Margaret baked bread every Sunday, just as her mother had done. The ritual connected her to generations of women who had come before.
The storm passed as quickly as it had arrived, leaving the air fresh and clean. Puddles reflected the clearing sky like scattered mirrors.
Night fell over the city, bringing with it a thousand points of light. From her apartment window, she could see the world coming alive.
The professor adjusted his glasses and began to speak. His words would change the way his students thought about everything.
Spring arrived early that year, catching everyone by surprise. Cherry blossoms bloomed weeks ahead of schedule, painting the streets pink and white.
In the monastery library, Brother Thomas carefully unfolded each quire of ancient parchment. The medieval manuscript revealed secrets that had been hidden for centuries.
The scribe worked by candlelight, preparing another quire for the illuminated text. Each sheet would become part of a book that would outlast its creator.
She inherited her great-aunt's collection of rare manuscripts, each quire telling a different story from the past. The weight of history felt heavy in her hands.
The restoration expert examined the damaged quire under magnification. Water had stained the edges, but the text remained surprisingly legible after all these years.
`
