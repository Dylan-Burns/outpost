import Link from "next/link";
export default function NotFound() {
  return (
    <div className="wrap narrow prose page-head">
      <h1>Off the map</h1>
      <p className="lede">There is no page at this address.</p>
      <p><Link href="/">Back to the start</Link></p>
    </div>
  );
}
