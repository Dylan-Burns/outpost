import type { MetadataRoute } from "next";
import { SITE } from "@/lib/site";
export default function sitemap(): MetadataRoute.Sitemap {
  return ["", "/install", "/catalog", "/docs"].map((p) => ({ url: SITE + p, changeFrequency: "weekly" }));
}
