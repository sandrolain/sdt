import { Suspense, lazy, useEffect, useMemo, useRef, useState } from "react";
import type { QtiAssessmentItemPlayerHandle } from "@longsightgroup/qti3-player-react";
import { fetchQTI, recordAttempt, type LessonBundle, type QtiResponses } from "../lib/learn";
import type { LessonItem } from "../lib/learn";

type ScoredResponse = { responses: QtiResponses; score: number };
const QtiAssessmentItemPlayer = lazy(() =>
  import("@longsightgroup/qti3-player-react").then((module) => ({
    default: module.QtiAssessmentItemPlayer,
  })),
);

/** One portable QTI item with local practice feedback and an explicit save action. */
export function QtiPracticeList({ lesson, items }: { lesson: LessonBundle; items: LessonItem[] }) {
  const [qtiItems, setQtiItems] = useState<Record<string, string> | null>(null);
  useEffect(() => {
    let cancelled = false;
    fetchQTI(lesson.concept)
      .then((bundle) => {
        if (!cancelled) setQtiItems(bundle.items);
      })
      .catch(() => {
        if (!cancelled) setQtiItems({});
      });
    return () => {
      cancelled = true;
    };
  }, [lesson.concept]);

  return (
    <ol>
      {items.map((item) => (
        <li key={item.id}>
          <QtiPracticeItem
            concept={lesson.concept}
            item={item}
            xml={qtiItems?.[item.id] ?? null}
            loading={qtiItems === null}
          />
        </li>
      ))}
    </ol>
  );
}

export function QtiPracticeItem({
  concept,
  item,
  xml,
  loading,
}: {
  concept: string;
  item: LessonItem;
  xml: string | null;
  loading: boolean;
}) {
  const playerRef = useRef<QtiAssessmentItemPlayerHandle>(null);
  const [scored, setScored] = useState<ScoredResponse | null>(null);
  const [recorded, setRecorded] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const loadOptions = useMemo(
    () => ({
      status: "interacting" as const,
      sessionControl: { validateResponses: true, showFeedback: true },
    }),
    [],
  );

  function checkAnswer() {
    setError(null);
    setRecorded(null);
    const result = playerRef.current?.scoreAttempt();
    if (!result) {
      setScored(null);
      setError("Complete the required response before checking it.");
      return;
    }
    const scoreValue = result.outcomes["SCORE"];
    const score = typeof scoreValue === "number" ? scoreValue : Number(scoreValue);
    if (!Number.isFinite(score)) {
      setScored(null);
      setError("The QTI item did not return a practice score.");
      return;
    }
    setScored({ responses: result.state.responses, score });
  }

  async function saveAttempt() {
    if (!scored || busy) return;
    setBusy(true);
    setError(null);
    try {
      const result = await recordAttempt(concept, item.id, scored.responses);
      setRecorded(result.grade);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="qti-practice" aria-labelledby={`qti-${item.id}`}>
      <h3 id={`qti-${item.id}`}>{xml ? `Question ${item.id}` : item.prompt}</h3>
      {!xml ? <p className="oc-muted">({item.type})</p> : null}
      {xml ? (
        <Suspense fallback={<p className="oc-muted">Loading interaction…</p>}>
          <QtiAssessmentItemPlayer
            ref={playerRef}
            xml={xml}
            loadOptions={loadOptions}
            onResponseChange={() => {
              setScored(null);
              setRecorded(null);
            }}
            onLoadError={() => setError("This QTI item could not be loaded for practice.")}
          />
        </Suspense>
      ) : (
        <p className="oc-muted">
          {loading
            ? "Loading practice items…"
            : "Interactive practice is unavailable for this item."}
        </p>
      )}
      {xml ? (
        <div className="qti-practice__actions">
          <button className="qti-practice__action" type="button" onClick={checkAnswer}>
            Check answer
          </button>
          {scored ? (
            <>
              <output aria-live="polite">
                Local practice score: {Math.round(scored.score * 100)}%
              </output>
              <button
                className="qti-practice__action"
                type="button"
                onClick={saveAttempt}
                disabled={busy || recorded !== null}
              >
                {busy ? "Recording…" : recorded ? "Attempt recorded" : "Record attempt"}
              </button>
            </>
          ) : null}
          {recorded ? <output aria-live="polite">Recorded by SDT as {recorded}.</output> : null}
        </div>
      ) : null}
      {error ? (
        <p className="qti-practice__error" role="alert">
          {error}
        </p>
      ) : null}
    </section>
  );
}
