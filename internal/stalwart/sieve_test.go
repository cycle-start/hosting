package stalwart

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateForwardScript_Empty(t *testing.T) {
	result := GenerateForwardScript(nil)
	assert.Equal(t, "", result)
}

func TestGenerateForwardScript_EmptySlice(t *testing.T) {
	result := GenerateForwardScript([]ForwardRule{})
	assert.Equal(t, "", result)
}

func TestGenerateForwardScript_SingleCopy(t *testing.T) {
	result := GenerateForwardScript([]ForwardRule{
		{Destination: "bob@gmail.com", KeepCopy: true},
	})
	expected := "require [\"copy\"];\nredirect :copy \"bob@gmail.com\";\n"
	assert.Equal(t, expected, result)
}

func TestGenerateForwardScript_SingleNoCopy(t *testing.T) {
	result := GenerateForwardScript([]ForwardRule{
		{Destination: "bob@gmail.com", KeepCopy: false},
	})
	expected := "redirect \"bob@gmail.com\";\n"
	assert.Equal(t, expected, result)
}

func TestGenerateForwardScript_Mixed(t *testing.T) {
	result := GenerateForwardScript([]ForwardRule{
		{Destination: "bob@gmail.com", KeepCopy: true},
		{Destination: "carol@yahoo.com", KeepCopy: false},
	})
	expected := "require [\"copy\"];\nredirect :copy \"bob@gmail.com\";\nredirect \"carol@yahoo.com\";\n"
	assert.Equal(t, expected, result)
}

func TestGenerateForwardScript_AllNoCopy(t *testing.T) {
	result := GenerateForwardScript([]ForwardRule{
		{Destination: "alice@example.com", KeepCopy: false},
		{Destination: "bob@example.com", KeepCopy: false},
	})
	expected := "redirect \"alice@example.com\";\nredirect \"bob@example.com\";\n"
	assert.Equal(t, expected, result)
}

func TestGenerateForwardScript_AllCopy(t *testing.T) {
	result := GenerateForwardScript([]ForwardRule{
		{Destination: "alice@example.com", KeepCopy: true},
		{Destination: "bob@example.com", KeepCopy: true},
	})
	expected := "require [\"copy\"];\nredirect :copy \"alice@example.com\";\nredirect :copy \"bob@example.com\";\n"
	assert.Equal(t, expected, result)
}
