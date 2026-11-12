package impact

import "testing"

// @verifies TC-OBL-015
func TestComputeMarkdownSpecDelta_TracksAddedTCsStructsAndFields(t *testing.T) {
	beforeMD := "# Shared Spec\n\n```go\npackage sample\ntype Alpha struct {\n\tID string\n}\n```\n\n| ID | Desc |\n| :--- | :--- |\n| `TC-SHR-01` | Base |\n"
	afterMD := "# Shared Spec\n\n```go\npackage sample\ntype Alpha struct {\n\tID string\n\tNewSiblingField bool\n}\ntype SiblingOnlyStruct struct {\n\tCount int\n}\n```\n\n| ID | Desc |\n| :--- | :--- |\n| `TC-SHR-01` | Base |\n| `TC-SHR-02` | Sibling vector |\n"

	delta := NewSiblingDelta()
	ComputeMarkdownSpecDelta(beforeMD, afterMD, delta)

	if delta.HasAddedTC("TC-SHR-01") {
		t.Fatal("did not expect pre-existing TC-SHR-01 in sibling delta")
	}
	if !delta.HasAddedTC("TC-SHR-02") {
		t.Fatal("expected newly added TC-SHR-02 in sibling delta")
	}
	if !delta.HasAddedStruct("SiblingOnlyStruct") {
		t.Fatal("expected SiblingOnlyStruct in sibling delta")
	}
	if !delta.HasAddedField("Alpha", "NewSiblingField") {
		t.Fatal("expected Alpha.NewSiblingField in sibling delta")
	}
	if delta.HasAddedField("Alpha", "ID") {
		t.Fatal("did not expect unchanged Alpha.ID in sibling delta")
	}
}
