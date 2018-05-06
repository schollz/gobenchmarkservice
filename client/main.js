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
  console.log(e);
  console.log(e.explicitOriginalTarget.id)
  var postRoute = "benchmark";
  if (e.explicitOriginalTarget.id == "gofmt-button") {
    postRoute = "fmt"
  }
  const value = document.getElementById("code-input").value;
  output = document.getElementById("code-output");
  output.innerHTML = "<progress/>";
  // TODO: use fetch below instead of this stub
  await new Promise(resolve => setTimeout(resolve, 1000));
  console.log(JSON.stringify({'code':document.getElementById("code-input").value}));
const response = await fetch("http://localhost:8080/"+postRoute, {
  method: "POST",
  mode: 'no-cors',
  headers: {
        'Content-Type': 'application/json'
    },
  body: JSON.stringify({'code':document.getElementById("code-input").value}),
}).then(function(response) {
    return response.json();
  })
  .then(function(myJson) {
    console.log(myJson);
  });
console.log(response.json());
  output.innerHTML = `You entered:
<pre>${value}</pre>
The output was
<pre>TODO</pre>
`;
}

window.onload = init;
