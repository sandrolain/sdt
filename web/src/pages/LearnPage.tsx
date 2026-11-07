import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { fetchCourse, fetchLesson, type CourseBundle, type LessonBundle } from "../lib/learn";
import { renderMarkdown } from "../lib/markdown";

/**
 * The learning surface: a course view and a lesson reader, read-only over the
 * corpus (decision 0043). The data comes from `/api/learn/*`, which runs the CLI;
 * the lesson bundle arrives without the grading key, and this page never renders
 * one anyway.
 */
export function LearnPage({ mode }: { mode: "course" | "lesson" }) {
  const params = useParams();
  const id = (params["*"] ?? "").replace(/^\/+/, "");
  const [course, setCourse] = useState<CourseBundle | null>(null);
  const [lesson, setLesson] = useState<LessonBundle | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setError(null);
    setCourse(null);
    setLesson(null);
    if (!id) {
      setError("No id in the address.");
      return;
    }
    const load = mode === "course" ? fetchCourse(id) : fetchLesson(id);
    load
      .then((data) => {
        if (cancelled) return;
        if (mode === "course") setCourse(data as CourseBundle);
        else setLesson(data as LessonBundle);
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      cancelled = true;
    };
  }, [id, mode]);

  if (error) {
    return (
      <main className="content">
        <p className="content__empty">{error}</p>
      </main>
    );
  }
  if (mode === "course") {
    return (
      <main className="content">
        {course === null ? (
          <p className="content__empty">Loading course…</p>
        ) : (
          <CourseView course={course} />
        )}
      </main>
    );
  }
  return (
    <main className="content">
      {lesson === null ? (
        <p className="content__empty">Loading lesson…</p>
      ) : (
        <LessonView lesson={lesson} />
      )}
    </main>
  );
}

/** Markdown is rendered by the corpus's own renderer, sanitized inside it: a
 * lesson's prose is markdown, and a reader that shows its asterisks is not one.
 * The trailing `source_span` line is stripped: the page prints the span itself. */
function Markdown({ text }: { text: string }) {
  const html = useMemo(
    () => renderMarkdown(text.replace(/\n?`source_span`:[^\n]*\s*$/, "")),
    [text],
  );
  return <div dangerouslySetInnerHTML={{ __html: html }} />;
}

function CourseView({ course }: { course: CourseBundle }) {
  const intake = course.intake ?? {};
  return (
    <article className="doc-view">
      <h1>{course.title ?? course.id}</h1>
      <p className="oc-muted">
        {intake.audience ? <>{intake.audience} · </> : null}
        {intake.from_level && intake.to_level ? (
          <>
            {intake.from_level} → {intake.to_level} ·{" "}
          </>
        ) : null}
        {course.lessons_ready ?? 0} of {course.lessons_total ?? 0} lessons ready
      </p>
      {intake.outcome ? <p>{intake.outcome}</p> : null}
      {(course.members ?? []).map((theme) => (
        <section key={theme.id}>
          <h2>{theme.title ?? theme.id}</h2>
          <ul>
            {(theme.members ?? []).map((concept) => {
              const material = course.materials?.[concept];
              return (
                <li key={concept}>
                  <Link to={`/learn/lesson/${concept}`}>{concept.split("/").pop()}</Link>
                  {material ? (
                    <>
                      {" · "}
                      <Link to={`/docs/${material.path}`}>{material.title ?? "Material"}</Link>
                    </>
                  ) : null}
                </li>
              );
            })}
          </ul>
        </section>
      ))}
    </article>
  );
}

function LessonView({ lesson }: { lesson: LessonBundle }) {
  const objectives = lesson.objectives ?? [];
  const items = lesson.items ?? [];
  return (
    <article className="doc-view">
      <h1>{lesson.title ?? lesson.concept}</h1>
      <p className="oc-muted">
        {lesson.difficulty ? <>{lesson.difficulty} · </> : null}
        {lesson.minutes ? <>{lesson.minutes} minutes · </> : null}
        {objectives.length} objective{objectives.length === 1 ? "" : "s"}
      </p>

      <h2>Objectives</h2>
      <ol>
        {objectives.map((objective) => (
          <li key={objective}>{objective}</li>
        ))}
      </ol>

      {lesson.prerequisites?.length ? (
        <>
          <h2>Prerequisites</h2>
          <p>{lesson.prerequisites.join(" · ")}</p>
        </>
      ) : null}

      {lesson.worked_example?.text ? (
        <>
          <h2>Worked example</h2>
          <Markdown text={lesson.worked_example.text.trim()} />
          {lesson.worked_example.source_span ? (
            <p className="oc-muted">
              <code>{lesson.worked_example.source_span}</code>
            </p>
          ) : null}
        </>
      ) : null}

      {items.length ? (
        <>
          <h2>Questions</h2>
          <ol>
            {items.map((item) => (
              <li key={item.id}>
                {item.prompt} <span className="oc-muted">({item.type})</span>
              </li>
            ))}
          </ol>
        </>
      ) : null}

      {lesson.transfer ? (
        <>
          <h2>Transfer</h2>
          <Markdown text={lesson.transfer} />
        </>
      ) : null}

      {lesson.material ? (
        <>
          <h2>Material</h2>
          <p>
            <Link to={`/docs/${lesson.material.path}`}>
              {lesson.material.title ?? lesson.material.path}
            </Link>
          </p>
        </>
      ) : null}

      {lesson.practice?.length ? (
        <>
          <h2>Practice</h2>
          {lesson.practice.map((note) => (
            <section key={note.path}>
              <h3>
                <Link to={`/docs/${note.path}`}>{note.title ?? note.path}</Link>
              </h3>
              {note.cases.map((practiceCase) => (
                <article key={practiceCase.title}>
                  <h4>{practiceCase.title}</h4>
                  <Markdown text={practiceCase.text} />
                </article>
              ))}
            </section>
          ))}
        </>
      ) : null}
    </article>
  );
}
