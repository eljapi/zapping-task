const MESSAGES = {
  invalid: "Invalid email or password.",
  fields: "Please enter a name and a valid email.",
  password: "Password must be at least 8 characters.",
  taken: "That email is already registered.",
};

const code = new URLSearchParams(location.search).get("error");

if (code) {
  const box = document.getElementById("error");
  box.textContent = MESSAGES[code] || "Something went wrong.";
  box.style.display = "block";
}
