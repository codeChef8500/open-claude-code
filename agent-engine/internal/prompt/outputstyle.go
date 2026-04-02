package prompt

import "strings"

// OutputStyle controls how the model is instructed to format its replies.
type OutputStyle string

const (
	// OutputStyleDefault lets the model choose its own formatting.
	OutputStyleDefault OutputStyle = ""
	// OutputStyleConcise asks for terse, minimal replies.
	OutputStyleConcise OutputStyle = "concise"
	// OutputStyleDetailed requests thorough, well-structured answers.
	OutputStyleDetailed OutputStyle = "detailed"
	// OutputStyleJSON requests a machine-readable JSON response.
	OutputStyleJSON OutputStyle = "json"
	// OutputStyleMarkdown explicitly requests GitHub-flavoured markdown.
	OutputStyleMarkdown OutputStyle = "markdown"
	// OutputStylePlainText strips all markdown/formatting.
	OutputStylePlainText OutputStyle = "plain_text"
)

// outputStyleInstructions maps each style to the injected instruction text.
var outputStyleInstructions = map[OutputStyle]string{
	OutputStyleConcise: `Respond concisely and directly. Omit preambles, summaries, and unnecessary ` +
		`elaboration. Prefer short sentences and bullet points where appropriate.`,
	OutputStyleDetailed: `Provide a thorough, well-structured response. Use headings, numbered lists, ` +
		`and code blocks where helpful. Explain your reasoning step by step.`,
	OutputStyleJSON: `Your entire response MUST be valid JSON. Do not include any text outside the ` +
		`JSON structure. Do not wrap the JSON in markdown code fences.`,
	OutputStyleMarkdown: `Format your response using GitHub-flavoured Markdown. Use headings, ` +
		`bold/italic, code blocks with language tags, and bullet/numbered lists as appropriate.`,
	OutputStylePlainText: `Respond in plain text only. Do not use markdown, HTML, or any other ` +
		`markup. Use plain punctuation for structure.`,
}

// OutputStyleInstruction returns the instruction string to append to the
// system prompt for the given output style, or "" for the default style.
func OutputStyleInstruction(style OutputStyle) string {
	return outputStyleInstructions[style]
}

// InjectOutputStyle appends the style instruction to a system prompt string
// and returns the result.  If style is OutputStyleDefault it returns prompt
// unchanged.
func InjectOutputStyle(prompt string, style OutputStyle) string {
	instr := OutputStyleInstruction(style)
	if instr == "" {
		return prompt
	}
	if prompt == "" {
		return instr
	}
	return strings.TrimRight(prompt, "\n") + "\n\n## Output Format\n\n" + instr
}
