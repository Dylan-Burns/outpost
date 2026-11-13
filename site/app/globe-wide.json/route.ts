import { globeData } from "@/lib/globe";

// The same outlines in finer detail, for the half globe a desktop shows as
// wide as its screen. Built once with the site.
export const dynamic = "force-static";

export function GET() {
  return Response.json(globeData("wide"));
}
