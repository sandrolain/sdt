import { fetchJSON } from "./fetchJson";

/**
 * The learning surface's data, answered by the viewer's `/api/learn/*` routes,
 * which in turn run the CLI (decision 0043). One shape per route, and the lesson
 * shape carries no grading key: the server strips it before the payload exists.
 */

export type CourseIntake = {
  audience?: string;
  from_level?: string;
  to_level?: string;
  outcome?: string;
};

export type CourseTheme = {
  order: number;
  id: string;
  title?: string;
  status?: string;
  members?: string[];
};

export type CourseBundle = {
  id: string;
  title?: string;
  status?: string;
  intake?: CourseIntake;
  members?: CourseTheme[];
  lessons_ready?: number;
  lessons_total?: number;
  materials?: Record<string, MaterialRef>;
};

export type MaterialRef = {
  path: string;
  title?: string;
  lang?: string;
};

export type LessonItem = {
  id: string;
  objective: number;
  type: string;
  prompt: string;
  claim?: string;
};

export type LessonBundle = {
  concept: string;
  title?: string;
  objectives?: string[];
  difficulty?: string;
  minutes?: number;
  prerequisites?: string[];
  worked_example?: { source_span?: string; text: string };
  items?: LessonItem[];
  claims?: { id: string; digest: string; text?: string }[];
  transfer?: string;
  material?: MaterialRef;
  missing?: string[];
};

/** fetchCourse reads one syllabus: themes, lessons and readiness. */
export function fetchCourse(syllabus: string): Promise<CourseBundle> {
  return fetchJSON<CourseBundle>(`/api/learn/course?syllabus=${encodeURIComponent(syllabus)}`);
}

/** fetchLesson reads one concept's teaching bundle, key-free. */
export function fetchLesson(concept: string): Promise<LessonBundle> {
  return fetchJSON<LessonBundle>(`/api/learn/lesson?concept=${encodeURIComponent(concept)}`);
}

/** courseOfLesson reports which syllabus a concept belongs to, when the corpus
 * says; today the course bundle is the only source, so this is a pure helper the
 * pages use once a course is loaded. */
export function lessonInCourse(course: CourseBundle, concept: string): boolean {
  return (course.members ?? []).some((theme) => (theme.members ?? []).includes(concept));
}

/** A learning view a document offers, when it is a lesson or a syllabus. */
export type LearnLink = { to: string; label: string };

/**
 * learnLinkForPath maps a corpus document path to the learning view it belongs to:
 * a syllabus opens the course, a lesson opens the reader. Everything else in
 * `context/learning/` (assessments, themes) has no view of its own.
 */
export function learnLinkForPath(path: string): LearnLink | null {
  const match = /^context\/learning\/(.+)$/.exec(path);
  if (!match) return null;
  const rel = match[1];
  if (rel.endsWith(".assessment.md") || rel.endsWith(".theme.md")) return null;
  if (rel.endsWith(".syllabus.md")) {
    return {
      to: `/learn/course/${rel.slice(0, -".syllabus.md".length)}`,
      label: "Apri il corso",
    };
  }
  if (!rel.endsWith(".md") || rel.startsWith("themes/") || rel.startsWith("syllabi/")) return null;
  return { to: `/learn/lesson/${rel.slice(0, -".md".length)}`, label: "Apri come lezione" };
}
