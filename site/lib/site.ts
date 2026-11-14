// Where Outpost lives.
//
// People type and paste the bare domain. It answers with a redirect to www,
// which is the address the pages are served from, so that is the one given
// to search engines and link previews. Anything that fetches from the bare
// domain (curl -L, the installer, a node) follows the redirect.
export const DOMAIN = "projectoutpost.org";
export const SITE = `https://www.${DOMAIN}`;

/** Where the source lives, for anyone who wants to read it or build it themselves. */
export const REPO = "https://github.com/Dylan-Burns/outpost";

/** The one line that installs Outpost. */
export const INSTALL = `curl -fsSL https://${DOMAIN}/install.sh | sudo bash`;

/** Ollama's own one-line installer, for the optional AI assistant. It works on a Mac and on Linux. */
export const GET_AI = "curl -fsSL https://ollama.com/install.sh | sh";
