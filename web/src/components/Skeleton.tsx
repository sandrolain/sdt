interface SkeletonLinesProps {
  count?: number;
  label?: string;
  className?: string;
}

/** Wireframe placeholder shown while an async surface loads. */
export function SkeletonLines({ count = 4, label = "Loading", className }: SkeletonLinesProps) {
  return (
    <div className={`skeleton${className ? ` ${className}` : ""}`} role="status" aria-label={label}>
      {Array.from({ length: count }, (_, i) => (
        <span key={i} className="skeleton__line" />
      ))}
    </div>
  );
}
