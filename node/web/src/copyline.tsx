import { useState } from "preact/hooks";
import { copyText } from "./format";

/** A line to paste into Terminal, with a button that copies it. */
export function CopyLine({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div class="copyline">
      <code class="kbd small">
        {/* One span per word, so a wrapped line breaks between flags and never inside one. */}
        {text.split(" ").map((word, i) => (
          <span key={i}>
            {i > 0 && " "}
            <span class="word">{word}</span>
          </span>
        ))}
      </code>
      <button
        type="button"
        class="btn small"
        onClick={async () => {
          setCopied(await copyText(text));
          setTimeout(() => setCopied(false), 1800);
        }}
      >
        {copied ? "Copied" : "Copy"}
      </button>
    </div>
  );
}
