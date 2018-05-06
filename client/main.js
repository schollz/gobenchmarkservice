let editor;
window.onload = init;

function init() {
  const textarea = document.getElementById("code-input");
  textarea.value = `package main

import (
  "fmt"
  "testing"
)

func BenchmarkFoo(b *testing.B) {
  for i := 0; i < b.N; i++ {
    fmt.Sprintf("hello, world")
  }
}


`;
  editor = CodeMirror.fromTextArea(textarea, {
    lineNumbers: true,
    mode: "javascript",
    autofocus: true,
    indentWithTabs: true,
    tabSize: 2,
    autoCloseBrackets: true,
    matchBrackets: true,
    mode: "text/x-go",
    showTrailingSpace: true,
    extraKeys: {
      "Shift-Enter": handleSubmit,
      "Ctrl-Space": handleFmt,
    },
  });
  const form = document.getElementById("code-form");
  form.onsubmit = handleSubmit;
  document.getElementById("gofmt-button").onclick = handleFmt;
}

async function handleFmt(e) {
  e && e.preventDefault && e.preventDefault();
  const value = editor.getValue();
  const output = document.getElementById("code-output");
  output.innerHTML = "Fmting... <progress/>";
  await new Promise(resolve => setTimeout(resolve, 100));
  const { code, success, message } = await fmt(value);
  if (success) {
    editor.setValue(code);
    output.innerHTML = "<p>formated!</p>";
  } else {
    output.innerHTML = `<p style='color:red;'>${message}</p>`;
  }
  editor.focus();
}

async function handleSubmit(e) {
  e && e.preventDefault && e.preventDefault();
  const value = editor.getValue();
  const output = document.getElementById("code-output");
  output.innerHTML = "Submitting... <progress/>";
  await new Promise(resolve => setTimeout(resolve, 100));
  const { message, success, benchmarks } = await benchmark(value);
  console.log(benchmarks);
  if (success) {
    output.innerHTML = `You entered:
  <pre>${value}</pre>
  The output was
  <pre>${JSON.stringify(message)}</pre>
  `;
  } else {
    output.innerHTML = `<p style="color:red;">${message}</p>`;
  }
}

/**
 * Runs `gofmt` on code.
 * @param {string} code
 * @returns {Promise<string>}
 */
async function fmt(code) {
  const response = await fetch("/fmt", {
    method: "POST",
    mode: "cors",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code }),
  });
  return await response.json();
}

/**
 * Runs `go test -bench=. --benchmem` on the code.
 * @param {string} code
 * @returns {Promise<object>}
 */
async function benchmark(code) {
  const response = await fetch("/run", {
    method: "POST",
    mode: "cors",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code }),
  });
  return await response.json();
}
