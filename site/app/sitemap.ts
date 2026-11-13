import type { MetadataRoute } from "next";
const base = "https://outpost-offline.vercel.app";
export default function sitemap(): MetadataRoute.Sitemap {
  return ["", "/install", "/catalog", "/docs"].map((p) => ({ url: base + p, changeFrequency: "weekly" }));
}
