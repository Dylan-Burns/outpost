import Link from "next/link";

export default function NotFound() {
  return (
    <div className="lost">
      <div className="wrap">
        <h1>No record</h1>
        <p className="lede">There is no page at this address.</p>
        <Link href="/" className="btn label">
          Back to the start
        </Link>
      </div>
    </div>
  );
}
