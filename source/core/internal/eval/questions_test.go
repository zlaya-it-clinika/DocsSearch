package eval

import "testing"

func TestLoad_QuestionsAndMatch(t *testing.T) {
	questions, err := Load_Questions()
	if err != nil {
		t.Fatal(err)
	}
	if len(questions) != 3 || questions[0].Paragraph != "5.1" || !questions[2].Fallback {
		t.Fatalf("questions %+v", questions)
	}
	if !Paragraph_Matches("5.1", "5.1") || !Paragraph_Matches("5.1.1", "5.1") || Paragraph_Matches("3.1.1", "5.1") || Paragraph_Matches("", "5.1") {
		t.Fatal("paragraph match")
	}
}
