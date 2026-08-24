package prompt

const promptTemplate = `
{{ .Preamble }}

You will receive:
1. Selected option profiles.
2. Receipt products prepared by the backend.
3. Backend response schema example.
4. Rules and response requirements.

Your task:
- Enrich only the products provided in receipt.products.
- Use only selected option profiles.
- Follow option profile descriptions and examples exactly.
- Return a backend-compatible JSON response.

Selected option profiles:
{{ .OptionsJSON }}

Rules:
{{ .RulesJSON }}

Backend response requirements:
{{ .RequirementsJSON }}

Receipt product input:
{{ .ReceiptJSON }}

Backend response schema example:
{{ .ResponseSchema }}

Final instruction:
Return only the final JSON object.
`
