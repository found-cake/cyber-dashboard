package body

import (
	"testing"

	"github.com/chromedp/cdproto"
)

func TestChromiumDocumentGuardTreatsExpiredInterceptionAsSettled(t *testing.T) {
	// Given Chrome reports that a paused request completed before it was continued.
	err := &cdproto.Error{Code: -32602, Message: "Invalid InterceptionId."}

	// When the document guard classifies the protocol response.
	settled := isSettledInterception(err)

	// Then the expired request does not invalidate an otherwise completed navigation.
	if !settled {
		t.Fatal("expired interception was treated as a navigation failure")
	}
}

func TestChromiumDocumentGuardPreservesOtherInvalidArgumentErrors(t *testing.T) {
	// Given Chrome rejects the request continuation for another invalid argument.
	err := &cdproto.Error{Code: -32602, Message: "Invalid request parameters"}

	// When the document guard classifies the protocol response.
	settled := isSettledInterception(err)

	// Then the unexpected protocol failure remains visible to the caller.
	if settled {
		t.Fatal("unexpected protocol failure was treated as a settled interception")
	}
}
