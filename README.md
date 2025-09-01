# OpenAI data export

A command-line tool to search and extract full conversations from an OpenAI ChatGPT data export (`conversations.json` inside the ZIP archive).  
Conversations matching your filters are written into structured folders, with attachments restored.

## Requesting your OpenAI data export

Before you can use `openai_extract`, you need to request your personal data export from OpenAI:

1. Go to [https://chat.openai.com](https://chat.openai.com) and log in.
2. Click on your profile picture → **Settings**.
3. Navigate to **Data controls**.
4. Click **Export data**.
5. Confirm the request. You’ll receive an email once your export is ready.
6. Download the `.zip` file from the email link.  
   This archive contains `conversations.json` and any uploaded files.

Use this `.zip` file as the input for `openai_extract`.

## Features

- Works directly on the exported `.zip` file (`conversations.json` + attachments).
- Search by one or more text patterns (**AND** semantics: all must match).
- Restrict results by:
  - Content type (e.g. `code`, `code_interpreter`)
  - Programming languages (detected from metadata and code fences).
- Outputs:
  - `conversation.json` (pretty-printed full conversation)
  - `files/` with any referenced attachments
- Each conversation gets its own folder, named by its start timestamp.

## Installation

Clone and build:

```bash
git clone https://github.com/yourname/openai_extract.git
cd openai_extract
go build -o openai_extract ./cmd/cli
```

This produces a binary `openai_extract`.

## Usage

```bash
openai_extract \
  -f <archive_file.zip> \
  -p <pattern> [-p <pattern> ...] \
  -o <output_folder> \
  [--content-type code,code_interpreter] \
  [--language python,go]
```

### Required flags

* `-f, --file` : Path to your OpenAI export `.zip`
* `-p, --pattern` : Search term or regex. Repeat `-p` to **AND** multiple patterns.
* `-o, --output` : Output folder where matched conversations are written.

### Optional filters

* `--content-type` : Require **all** of these content types. Example:

  ```bash
  --content-type code,code_interpreter
  ```
* `-l, --language` : Require **all** of these languages. Example:

  ```bash
  -l go -l python
  ```

### Examples

Match conversations containing both *feedback* and *service*:

```bash
openai_extract \
  -f export.zip \
  -p feedback -p service \
  -o assets/output
```

Match conversations that contain both patterns **and** include **Go** and **JavaScript** code:

```bash
openai_extract \
  -f export.zip \
  -p feedback -p service \
  -o assets/output \
  -l go -l javascript \
  --content-type code
```

## Output structure

```
assets/output/
  090125-1836/                # folder name from conversation start time
    conversation.json          # full conversation (pretty JSON)
    files/                     # any linked attachments
      image.png
      dataset.csv
```

## Development

Run vet/tests:

```bash
go vet ./...
go test ./...
```

## Notes

* Pattern matching is case-insensitive by default unless you pass explicit regex.
* Every filter (pattern, language, content-type) is **ANDed**. Each extra filter makes the match more restrictive.
* Designed for local use; no API calls.

## License

openai_extract is released under the [MIT License](MIT-LICENSE).

