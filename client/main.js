function init() {
  const textarea = document.getElementById("code-input");
  textarea.value = `package main

import "fmt"

func main() {
  fmt.Println("hello")
}
`;
  CodeMirror.fromTextArea(textarea, {
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
    },
  });
  const form = document.getElementById("code-form");
  form.onsubmit = handleSubmit;
}

async function handleSubmit(e) {
  e.preventDefault && e.preventDefault();
  const value = document.getElementById("code-input").value;
  output = document.getElementById("code-output");
  output.innerHTML = "<progress/>";
  // TODO: use fetch below instead of this stub
  await new Promise(resolve => setTimeout(resolve, 1000));
  /*
const response = await fetch("http://SERVERADDRESS:8080/benchmark", {
  method: "POST",
  body: new FormData(document.getElementById('code-form'))
});
  */
  output.innerHTML = `You entered:
<pre>${value}</pre>
The output was
<pre>TODO</pre>
`;
}

window.onload = init;
