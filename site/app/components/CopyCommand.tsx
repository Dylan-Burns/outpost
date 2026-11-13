"use client";
import { useState } from "react";

export default function CopyCommand({ command, label = "Copy" }: { command: string; label?: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="command">
      <code>
        <span className="prompt" aria-hidden="true">$ </span>
        {command}
      </code>
      <button
        type="button"
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(command);
            setCopied(true);
            setTimeout(() => setCopied(false), 1800);
          } catch {
            /* clipboard blocked: the text is still selectable */
          }
        }}
        aria-label={copied ? "Copied" : `Copy command: ${command}`}
      >
        {copied ? "Copied" : label}
      </button>
    </div>
  );
}
