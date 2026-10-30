import { useEffect } from "react";
import { Link, useNavigate } from "react-router";

import type { Route } from "./+types/home";
import { BUTTON_VARIANT_CLASSES } from "../components/button";
import { Logo } from "../components/logo";
import { useAuth } from "../lib/auth";

export function meta({}: Route.MetaArgs) {
  return [
    { title: "Shelfmark — Dependency consolidation for platform teams" },
    {
      name: "description",
      content:
        "Scanners tell you what's vulnerable. Shelfmark tells you what's redundant.",
    },
  ];
}

const POINTS = [
  {
    num: "01",
    title: "Capabilities",
    body: "Every library tagged against a curated taxonomy. Redundancy becomes something you can count.",
  },
  {
    num: "02",
    title: "Health",
    body: "Last release, maintainers, licence, archived status — for the libraries you depend on directly.",
  },
  {
    num: "03",
    title: "Cost of change",
    body: "Direct or transitive, runtime or test, and how many projects a consolidation would touch.",
  },
];

export default function Home() {
  const { me, loading } = useAuth();
  const navigate = useNavigate();

  // The landing renders at once for everyone; a signed-in user is sent on to
  // the app as soon as /api/me answers. replace keeps Back off the landing.
  useEffect(() => {
    if (!loading && me) {
      navigate("/dashboard", { replace: true });
    }
  }, [loading, me, navigate]);

  return (
    <div className="flex min-h-screen flex-col">
      <header className="pt-12 sm:pt-16 lg:pt-20">
        <div className="mx-auto w-full max-w-[44rem] px-4">
          <div className="mb-10 flex items-center justify-between">
            <Logo />
            <Link to="/login" className={BUTTON_VARIANT_CLASSES.secondary}>
              Zaloguj
            </Link>
          </div>

          <h1 className="mb-6 max-w-[20ch] font-serif text-[clamp(2rem,7vw,3.25rem)] leading-[1.12] tracking-[-.02em]">
            Scanners tell you what&rsquo;s vulnerable.{" "}
            <em className="font-normal text-accent not-italic">
              Shelfmark tells you what&rsquo;s redundant.
            </em>
          </h1>

          <p className="mb-10 max-w-[48ch] text-[clamp(1.05rem,2.4vw,1.2rem)] text-ink-dim">
            A catalog that classifies every open source library your
            organization depends on by what it actually does — so you can see
            where five libraries are doing one job, and decide which one
            stays.
          </p>
        </div>
      </header>

      <main>
        <div className="mx-auto w-full max-w-[44rem] px-4">
          <ul className="mb-12 grid gap-7 border-t border-rule pt-8 sm:grid-cols-3 sm:gap-8">
            {POINTS.map((point) => (
              <li key={point.num}>
                <span className="mb-[.6rem] block text-[.7rem] tracking-[.12em] text-ink-faint">
                  {point.num}
                </span>
                <h2 className="mb-1 font-serif text-[.95rem] font-semibold text-ink">
                  {point.title}
                </h2>
                <p className="text-[.92rem] leading-[1.55] text-ink-dim">
                  {point.body}
                </p>
              </li>
            ))}
          </ul>

          <div className="mb-16 border-t border-rule pt-8">
            <p className="mb-5 max-w-[46ch] text-[.95rem] text-ink-dim">
              Built on the SBOMs you already produce. Runs on your own
              infrastructure — your dependency data never leaves it.
            </p>
            <a
              className="inline-block rounded-full bg-accent px-6 py-3 text-[.95rem] font-medium text-bg transition hover:-translate-y-px hover:shadow-[0_6px_20px_-6px_color-mix(in_srgb,var(--color-accent)_50%,transparent)] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-[3px] focus-visible:outline-accent"
              href="mailto:hello@shelfmark.online?subject=Shelfmark"
            >
              Get in touch
            </a>
          </div>
        </div>
      </main>

      <footer className="mt-auto border-t border-rule py-8 text-[.8rem] text-ink-faint">
        <div className="mx-auto flex w-full max-w-[44rem] flex-wrap justify-between gap-x-6 gap-y-2 px-4">
          <span>Shelfmark</span>
          <a className="text-ink-faint" href="mailto:hello@shelfmark.online">
            hello@shelfmark.online
          </a>
        </div>
      </footer>
    </div>
  );
}
