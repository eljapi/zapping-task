const MESSAGES_URL = "/chat/messages";
const SEND_URL = "/chat";
const POLL_INTERVAL = 3000;

const list = document.getElementById("chat-messages");
const form = document.getElementById("chat-form");
const input = document.getElementById("chat-input");

function renderMessages(messages) {
  list.innerHTML = "";
  for (const message of messages) {
    const item = document.createElement("li");

    const author = document.createElement("span");
    author.className = "author";
    author.textContent = message.author;

    item.appendChild(author);
    item.appendChild(document.createTextNode(message.text));
    list.appendChild(item);
  }
  list.scrollTop = list.scrollHeight;
}

async function loadMessages() {
  const response = await fetch(MESSAGES_URL);
  if (!response.ok) return;
  renderMessages(await response.json());
}

form.addEventListener("submit", async (event) => {
  event.preventDefault();

  const text = input.value.trim();
  if (!text) return;

  const response = await fetch(SEND_URL, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({ text }),
  });

  if (response.ok) {
    input.value = "";
    loadMessages();
  }
});

loadMessages();
setInterval(loadMessages, POLL_INTERVAL);
