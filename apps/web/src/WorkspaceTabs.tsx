import { useId, useRef, useState, type ReactNode } from "react";

export function WorkspaceTabs({
  sections,
}: {
  sections: { id: string; label: string; content: ReactNode }[];
}) {
  const prefix = useId();
  const [active, setActive] = useState(0);
  const tabs = useRef<(HTMLButtonElement | null)[]>([]);
  return (
    <div className="project-workspace">
      <div
        role="tablist"
        aria-label="Project sections"
        className="workspace-tabs"
      >
        {sections.map((section, index) => (
          <button
            key={section.id}
            ref={(element) => {
              tabs.current[index] = element;
            }}
            type="button"
            role="tab"
            id={`${prefix}-tab-${section.id}`}
            aria-controls={`${prefix}-panel-${section.id}`}
            aria-selected={active === index}
            tabIndex={active === index ? 0 : -1}
            onClick={() => setActive(index)}
            onKeyDown={(event) => {
              let next: number;
              switch (event.key) {
                case "ArrowRight":
                  next = (index + 1) % sections.length;
                  break;
                case "ArrowLeft":
                  next = (index - 1 + sections.length) % sections.length;
                  break;
                case "Home":
                  next = 0;
                  break;
                case "End":
                  next = sections.length - 1;
                  break;
                default:
                  return;
              }
              event.preventDefault();
              setActive(next);
              tabs.current[next]?.focus();
            }}
          >
            {section.label}
          </button>
        ))}
      </div>
      {sections.map((section, index) => (
        <div
          key={section.id}
          role="tabpanel"
          id={`${prefix}-panel-${section.id}`}
          aria-labelledby={`${prefix}-tab-${section.id}`}
          hidden={active !== index}
          tabIndex={0}
        >
          {/* Keep drafts and in-flight execution state in memory across tabs. */}
          {section.content}
        </div>
      ))}
    </div>
  );
}
