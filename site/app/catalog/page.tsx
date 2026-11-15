import type { Metadata } from "next";
import ContentIndex from "@/app/components/ContentIndex";
import { bytes } from "@/lib/catalog";
import { INDEX_BASE, loadIndexRoot } from "@/lib/contentIndex";

export const metadata: Metadata = {
  title: "Content",
  description: "Every book, PDF, video, article, map region and AI model Outpost can install, with its size, length, date and why it is in the library.",
};

const num = (n: number) => n.toLocaleString("en-US");

export default function CatalogPage() {
  const root = loadIndexRoot();
  if (!root) {
    return (
      <div className="page-head">
        <div className="wrap">
          <h1>Content</h1>
          <p className="lede">The content index is being rebuilt. Please check back shortly.</p>
        </div>
      </div>
    );
  }
  const t = root.totals;
  return (
    <>
      <div className="page-head">
        <div className="wrap">
          <h1>Content</h1>
          <p className="lede">
            Everything an Outpost can hold, down to the single book, PDF, video and article. Open a subject, then a collection, then its
            shelves. Every row says what it is, how long it is, how big it is and when it was last updated; every collection says why it is
            in the library.
          </p>
          <dl className="titleblock">
            <div>
              <dt>Items</dt>
              <dd>
                {num(t.items)}
                <small>in {num(t.downloads)} downloads</small>
              </dd>
            </div>
            <div>
              <dt>Largest library</dt>
              <dd>
                {bytes(t.largestWithEverything)}
                <small>everything selected at once</small>
              </dd>
            </div>
            <div>
              <dt>Map regions</dt>
              <dd>
                {num(t.mapRegions)}
                <small>{bytes(t.mapBytes)} in all</small>
              </dd>
            </div>
            <div>
              <dt>Index built</dt>
              <dd>
                {root.generated.slice(0, 10)}
                <small>catalog sequence {root.catalogSequence}</small>
              </dd>
            </div>
          </dl>
        </div>
      </div>
      <section className="ix-section-wrap">
        <div className="wrap">
          <ContentIndex root={root} base={root.base || INDEX_BASE} />
          <div className="ix-sums">
            <h2 className="label">How the sizes add up</h2>
            <table className="cat-items">
              <tbody>
                <tr>
                  <td>{t.largestLibraryRule}</td>
                  <td className="num">{bytes(t.largestLibrary)}</td>
                </tr>
                <tr>
                  <td>All {num(t.mapRegions)} map regions, with the world overview, fonts and icons</td>
                  <td className="num">{bytes(t.mapBytes)}</td>
                </tr>
                <tr>
                  <td>All {num(t.models)} AI models</td>
                  <td className="num">{bytes(t.modelBytes)}</td>
                </tr>
                <tr>
                  <td>
                    <strong>Largest library you can install</strong>
                  </td>
                  <td className="num">
                    <strong>{bytes(t.largestWithEverything)}</strong>
                  </td>
                </tr>
                <tr>
                  <td className="quiet">
                    Every file in the catalog, counting each alternative edition separately (the six editions of Wikipedia, the Gutenberg
                    shelves that the full Gutenberg contains)
                  </td>
                  <td className="num quiet">{bytes(t.everyFile)}</td>
                </tr>
              </tbody>
            </table>
            <p className="quiet">
              Sizes are of the files as published, in decimal units (1 GB = 1,000,000,000 bytes). An item&apos;s size is its own text, document
              or video, without the pictures and page styles it shares with the rest of its collection.
              {t.unmeasured > 0 && ` ${num(t.unmeasured)} items are listed but not yet measured; their size reads “not yet measured”.`}
            </p>
          </div>
        </div>
      </section>
    </>
  );
}
