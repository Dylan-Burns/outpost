"use client";
import Link from "next/link";
import { usePathname } from "next/navigation";

const LINKS = [
  ["/catalog", "Content"],
  ["/docs", "Help"],
] as const;

export default function NavLinks() {
  const path = usePathname();
  return (
    <nav aria-label="Main">
      {LINKS.map(([href, label]) => (
        <Link key={href} href={href} aria-current={path === href ? "page" : undefined}>
          {label}
        </Link>
      ))}
      <Link href="/install" className="btn" aria-current={path === "/install" ? "page" : undefined}>
        Install
      </Link>
    </nav>
  );
}
