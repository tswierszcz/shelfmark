import type { ReactNode } from "react";

import { Logo } from "../components/logo";
import { Button } from "../components/button";
import { Badge } from "../components/badge";

function Section({
  title,
  caption,
  children,
}: {
  title: string;
  caption: string;
  children: ReactNode;
}) {
  return (
    <section className="mt-6 rounded-lg border border-rule bg-bg-soft p-4">
      <h2 className="text-lg font-semibold font-serif">{title}</h2>
      <p className="mt-1 text-xs text-ink-faint">{caption}</p>
      <div className="mt-3">{children}</div>
    </section>
  );
}

export default function KitchenSinkRedundancy() {
  return (
    <main className="mx-auto max-w-4xl p-6">
      <Logo className="mb-6" />
      <h1 className="text-xl font-semibold font-serif">Redundancy view — kitchen sink</h1>
      <p className="mt-1 text-sm text-ink-dim">
        Reference page for the 7 UI states used across the redundancy report view. No auth
        guard, no network calls — all data below is static fixture data.
      </p>

      <Section title="1. Default" caption="Normal button and a populated report row.">
        <div className="space-y-4">
          <Button type="button">Upload</Button>
          <table className="text-left text-sm">
            <tbody>
              <tr className="border-b border-rule/50">
                <td className="py-2 pr-4 font-mono">pkg:npm/left-pad@1.3.0</td>
                <td className="py-2 pr-4">Yes</td>
                <td className="py-2 pr-4">left-pad/left-pad</td>
                <td className="py-2 pr-4">
                  <span className="flex flex-wrap items-center gap-2">
                    <span>Utilities</span>
                    <Badge tone="accent">Strategic</Badge>
                  </span>
                </td>
                <td className="py-2 pr-4">—</td>
              </tr>
            </tbody>
          </table>
        </div>
      </Section>

      <Section
        title="2. Hover"
        caption="Mouse over the button or table row below to see the hover state."
      >
        <div className="space-y-4">
          <Button type="button">Upload</Button>
          <table className="text-left text-sm">
            <tbody>
              <tr className="border-b border-rule/50 hover:bg-bg">
                <td className="py-2 pr-4 font-mono">pkg:npm/is-odd@3.0.1</td>
                <td className="py-2 pr-4">No</td>
                <td className="py-2 pr-4">i-is/odd</td>
                <td className="py-2 pr-4">
                  <span className="flex flex-wrap items-center gap-2">
                    <span>Legacy</span>
                    <Badge tone="destructive">Retire candidate</Badge>
                  </span>
                </td>
                <td className="py-2 pr-4">
                  <span className="text-destructive">Retire?</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </Section>

      <Section
        title="3. Focus-visible"
        caption="Tab into the button below to see the focus-visible state."
      >
        <Button type="button">Upload</Button>
      </Section>

      <Section title="4. Disabled" caption="Button in its disabled state.">
        <Button type="button" disabled>
          Uploading…
        </Button>
      </Section>

      <Section title="5. Error" caption="The shared error-text pattern used across the view.">
        <p className="text-destructive text-sm">Upload failed: unsupported file type.</p>
      </Section>

      <Section
        title="6. Empty"
        caption="The empty-state table row shown when a project has no libraries."
      >
        <table className="w-full text-left text-sm">
          <tbody>
            <tr>
              <td colSpan={5} className="py-4 text-center text-ink-faint">
                No libraries recorded for this project.
              </td>
            </tr>
          </tbody>
        </table>
      </Section>

      <Section
        title="7. Loading"
        caption="The batch-progress chip pattern shown while a batch is processing."
      >
        <div className="text-sm">
          <p>
            Batch <span className="font-mono">fixture-batch-id</span> — processing…
          </p>
          <ul className="mt-1 flex flex-wrap gap-3">
            <li className="rounded border border-rule px-2 py-1">categorized: 2</li>
            <li className="rounded border border-rule px-2 py-1">processing…: 3</li>
          </ul>
        </div>
      </Section>
    </main>
  );
}
