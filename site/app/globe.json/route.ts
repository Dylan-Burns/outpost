import { globeData } from "@/lib/globe";

// Built once with the site: the outlines only change when the catalog does.
// This is the cut for the small whole globe a phone shows.
export const dynamic = "force-static";

export function GET() {
  return Response.json(globeData("small"));
}
