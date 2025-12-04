# 🚀 repl-reqs: The Interactive, Go-Powered HTTP Client REPL

**repl-reqs** is a powerful command-line utility, written in **Go**, that transforms your API workflow. It provides a full **Read-Evaluate-Print Loop (REPL)** interface, allowing you to draft, send, script, and manage HTTP requests with unparalleled flexibility all from your terminal.

---

## ✨ Why repl-reqs?

We move beyond simple HTTP clients by treating your API endpoints as native, **structured commands**. This enables advanced features like hierarchical commands, environment management, and provider-level configuration.

---

## 🎯 Key Features

### 1. Command-Line API Interaction (Hierarchical Commands)
* **Structured Access:** Treat your APIs as native commands (e.g., `list users`, `get document`).
* **Easy Parameter Supply:** Effortlessly supply parameters for query strings, URL segments, and the request body directly as 'command parameters'.
* **Intelligent Auto-Suggestions:** Full auto-suggestions for commands, parameters, and variable names, drastically speeding up your workflow.

### 2. Provider Configuration Framework
* **Cloud Provider Ready:** `repl-reqs` includes a robust configuration framework that allows cloud and service providers to package their APIs as structured CLI tools—similar to the powerful command-line tools provided by AWS or Azure.

### 3. Workflow & Efficiency
* **Scripting Simplified:** Create command sequences in both interactive (live) and non-interactive modes, supported by auto-suggestions. This allows you to execute multiple commands in a sequence with a single command, where each command has access to results from any of the commands that execute before it at any step of the sequence, even including the ability to logically filter out elements and their values from array like structures.
* **Dynamic Context:** Robust support for multiple **environments, variables, and value expansion (interpolation)** to handle dynamic data effortlessly.
* **Developer Experience:** The core **REPL environment is more than sufficient**, allowing you to draft and execute requests without ever switching context. However, for users who prefer to review and edit the overall structure of their request in a dedicated editor, we provide seamless integration. Simply use the **`$edit req`** command, which will instantly open the current request draft in **TOML format** within your configured (or default) terminal or GUI editor. Saving the file automatically updates your currently drafting request within `repl-reqs`. This feature provides complete flexibility and control for large-scale request editing and review.
* **Validation Framework:** Built-in tools for validating request payloads (query, URL params, and body) to ensure data integrity and error reporting even before the actual request is sent. (Another add-on feature for Providers)

### 4. Task Management
* **Background Tasks:** Send long-running tasks or requests to the background and seamlessly track their status.
* **Command Modes:** Each command that accepts arguments, if triggered without any arguments will result in setting that command as the **current command mode**. This way you can avoid repetitive typing. For instance, if you just type `set` without any subcommands or arguments, the handler will recognize that you want to get into `set` mode. Now all the other sub-commands of **$set** are available without the '$set' prefix.

### 5. Syntax Highlighting
* Responses are syntax highlighted for enhanced readability, making complex data structures easy to parse at a glance. 

---

## ⚙️ Installation

### Via Go
```bash
go install github.com/shubm-quodes/repl-reqs@latest
```


### Binary (Recommended)

Download the latest binary for your operating system from the [Releases page](https://github.com/shubm-quodes/repl-reqs/releases).

### 🚀 Getting Started
Start the REPL:

```bash
repl-reqs
```
### A Simple GET Request:

You can start drafting a request by using the '$draft_req' system command, Notice the default prompt 'repl-reqs',
the prompt also indicates the currently active env, which is 'Global' in this case.
```
repl-reqs (Global) 😼> $draft_req 

Request Draft (1) (Global) 😼> # prompt changed indicating a new request being drafted
```
Now get into 'set' mode, the prompt changes from 'repl-reqs' to '$set', indicating the change of mode. It's not necessary to get into the mode itself, you could also just simply fire the $set command along with any of it's sub commands like url, query etc.. but getting into the mode avoids repetitive typing. 
```
$set (Global)😼> $set

$set (Global)😼> url https://api.some.com/users/list
...some.com/users/list😼> #here the prompt changed to indicate current request draft's URL
```
Set a simple query parameter id with value as 1, in case you want to set multiple query parameters without repetitively typing 'query keyX valueX', just type 'query' and hit enter to get into the 'query mode'. Once done you can switch back to the previous mode by exiting out of the current mode by simply pressing ctrl+d.
```
...some.com/users/list😼> query id 1 #set query param with key as id and value as 1, equivalent to id=1
```
Exit out of set mode and send your drafted request!
```
 ...some.com/users/list😼> exit 

repl-reqs (Global) 😼> $send # send the drafted http request.
✅ Task completed (in: 200.96ms)
 {
  "success": true,
  "user": {
    "id": 1,
    "attributes": [
      "night owl",
      "avid reader"
    ],
    "firstName": "John",
    "lastName": "Doe"
  }
}

200 OK
repl-reqs (Global) 😼>
```
You could also save this request and give it a command name, if delimited by spaces each word will be considered as a sub command, this allows you to easily group related requests under a category like 'list users', 'list resources', 'list documents' and so on.
```
repl-reqs (Global) 😼> $save list users 

repl-reqs (Global) 😼> list users id=2 # same drafted request now available as a command with "auto-suggestable" query parameter which was set earlier, auto suggestions also available for even request body and url params.
✅ Task completed (in: 200.96ms) # Same request now available direclty as a commad
 {
  "success": true,
  "user": {
    "id": 1,
    "attributes": [
      "night owl",
      "avid reader"
    ],
    "firstName": "John",
    "lastName": "Doe"
  }
}

200 OK
repl-reqs (Global) 😼>
```

## **Automating Workflows with Record Mode**

Record Mode is a powerful feature designed to **automate repetitive and multi-step workflows**, drastically boosting your efficiency.

### **The Access Token Example**

Consider a common scenario: managing API requests that require an **access token**. Manually calling a login endpoint and copying the token into multiple requests is time-consuming and error-prone.

With Record Mode, you can automate this entire process by creating a sequence that:

* Calls the **login endpoint**.
* **Extracts** the access token from the response data.
* **Stores** the token as a persistent environment variable.
* Allows all subsequent API endpoints to **automatically reference and use** this updated token.

---

### **💡 Live Mode: Interactive Recording**

To make the workflow creation process even more efficient, Record Mode includes a **'Live Mode'**.

In Live Mode, when you trigger a command on a step, that command is **immediately executed**. This provides an **interactive environment** where you can **analyze the output of each command in real time** as you record the sequence. This means you don't have to finish and play the entire sequence to notice and fix any issues, allowing for **instant debugging** and a much faster workflow development cycle.

---

### **Beyond Variables: Dynamic Workflows**

While excellent for setting variables, Record Mode's true power lies in its ability to **streamline any multi-step, dynamic workflow**. Within a recorded sequence, each step benefits from a sophisticated data-referencing capability:

* A step can easily refer to the result of the **immediately preceding step**.
* Crucially, it can also access the output of **any earlier parent step** within the sequence.

This flexibility makes Record Mode the essential tool for building **dynamic, robust, and fully automated API workflows**.
Below is a simple example of how can you use the record mode -

```
repl-reqs (Global) 😼>$rec login_flow

Registered new sequence 'login_flow'

rec(login_flow) 🔴 step #1 (Global) 😼>login user email=someone@something.com password=******** # A login request saved as 'login user' command and two parameters that would go in request's body, repl-reqs automatically picks up these parameter email & password when you draft and save the request

rec(login_flow) 🔴 step #2 (Global) 😼>$set var accessToken {{$1.accessToken}}

rec(login_flow) 🔴 step #3 (Global) 😼>$finalize

sequence 'login_flow' saved successfully! 👌🏼

repl-reqs (Global) 😼>$play login_flow

repl-reqs (Global) 😼>$ls vars

📦 Variables - (In currently active environment: 'Global')

1. accessToken: <TOKEN> # variable now available in the current env.
```
