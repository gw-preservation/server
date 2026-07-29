package gameservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEmoteByCommand_Exists(t *testing.T) {
	emote, ok := GetEmoteByCommand("wave")
	assert.True(t, ok)
	assert.Equal(t, 806608701, emote.datEmoteId)
	assert.Equal(t, 0x7bd, emote.datStringId)
}

func TestGetEmoteByCommand_NotExists(t *testing.T) {
	_, ok := GetEmoteByCommand("nonexistent")
	assert.False(t, ok)
}

func TestGetEmoteByCommand_Duplicates(t *testing.T) {
	emote1, ok1 := GetEmoteByCommand("boo")
	assert.True(t, ok1)

	emote2, ok2 := GetEmoteByCommand("scare")
	assert.True(t, ok2)

	assert.Equal(t, emote1.datEmoteId, emote2.datEmoteId)
	assert.Equal(t, emote1.datStringId, emote2.datStringId)
}

func TestGetEmoteByCommand_DuplicateAliases(t *testing.T) {
	aliases := map[string]string{
		"boo":         "scare",
		"head":        "bowhead",
		"catchbreath": "breath",
		"drums":       "drum",
		"encourage":   "goteam",
		"airguitar":   "guitar",
		"five":        "highfive",
		"rude":        "taunt",
		"scis":        "scissors",
	}

	for alias, canonical := range aliases {
		emoteAlias, ok1 := GetEmoteByCommand(alias)
		assert.True(t, ok1, "alias %q not found", alias)

		emoteCanonical, ok2 := GetEmoteByCommand(canonical)
		assert.True(t, ok2, "canonical %q not found", canonical)

		assert.Equal(t, emoteCanonical.datEmoteId, emoteAlias.datEmoteId,
			"alias %q has different datEmoteId than %q", alias, canonical)
		assert.Equal(t, emoteCanonical.datStringId, emoteAlias.datStringId,
			"alias %q has different datStringId than %q", alias, canonical)
	}
}

func TestGetEmoteByCommand_AllEmotesExist(t *testing.T) {
	emoteNames := []string{
		"agree", "attention", "beckon", "beg", "bored", "bow", "bowhead",
		"breath", "cheer", "congrates", "cough", "dance", "doh", "doubletake",
		"drum", "clap", "fistshake", "flex", "flute", "goteam", "guitar",
		"helpme", "highfive", "jump", "kneel", "laugh", "moan", "no",
		"pickme", "point", "ponder", "pout", "rock", "ready", "roar",
		"salute", "scratch", "shoo", "sigh", "sorry", "taunt", "voilin",
		"wave", "yawn", "yes",
	}
	for _, name := range emoteNames {
		_, ok := GetEmoteByCommand(name)
		assert.True(t, ok, "emote %q not found", name)
	}
}
