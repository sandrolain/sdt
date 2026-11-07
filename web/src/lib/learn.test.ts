import { describe, expect, it } from "vitest";
import { learnLinkForPath, lessonInCourse, type CourseBundle } from "./learn";

describe("learnLinkForPath", () => {
  it("maps a syllabus to the course view", () => {
    expect(learnLinkForPath("context/learning/syllabi/software-architect.syllabus.md")).toEqual({
      to: "/learn/course/syllabi/software-architect",
      label: "Apri il corso",
    });
  });

  it("maps a lesson to the reader", () => {
    expect(learnLinkForPath("context/learning/architecture/four-key-metrics.md")).toEqual({
      to: "/learn/lesson/architecture/four-key-metrics",
      label: "Apri come lezione",
    });
  });

  it("gives no view to an assessment, a theme or a page outside learning", () => {
    expect(
      learnLinkForPath("context/learning/architecture/four-key-metrics.assessment.md"),
    ).toBeNull();
    expect(learnLinkForPath("context/learning/themes/delivery-metrics.theme.md")).toBeNull();
    expect(learnLinkForPath("context/wiki/architecture/four-key-metrics.md")).toBeNull();
  });
});

describe("lessonInCourse", () => {
  it("reports membership through the theme's members", () => {
    const course: CourseBundle = {
      id: "syllabi/x",
      members: [{ order: 1, id: "themes/t", members: ["architecture/a"] }],
    };
    expect(lessonInCourse(course, "architecture/a")).toBe(true);
    expect(lessonInCourse(course, "architecture/b")).toBe(false);
  });
});
