import { memo } from "react";
import { useAgentos } from "../AgentosContext";
import type { Session } from "../types";
import { Ago } from "./Ago";
import { CleanupControls } from "./CleanupControls";
import { Icon } from "./Icon";
import { PRMark } from "./PRMark";
import { StateDot } from "./StateDot";

type ThreadRowProps = { session: Session; selected: boolean; cursor: boolean; projectName?: string };

function ThreadRowView({ session, selected, cursor, projectName }: ThreadRowProps) {
  const { select, focus, dismissSession, report } = useAgentos(
    "select",
    "focus",
    "dismissSession",
    "report",
  );
  const ended = session.state === "ended";

  return (
    <div
      className="row row-inset group flex-col"
      data-state={session.state}
      data-selected={selected}
      data-cursor={cursor}
      style={{ opacity: ended ? 0.6 : 1 }}
    >
      <div className="flex w-full items-center">
        <button
          className="flex min-w-0 flex-1 flex-col gap-0.5 py-1.5 pr-2 pl-3 text-left"
          aria-current={selected}
          disabled={ended}
          title={ended ? `${session.title} (ended)` : session.title}
          style={ended ? { cursor: "default", opacity: 1 } : undefined}
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => {
            select(session.id);
            focus("terminal");
          }}
        >
          <span className="flex w-full items-center gap-2">
            <StateDot state={session.state} />
            <span
              className={`min-w-0 flex-1 truncate text-body ${selected ? "font-semibold" : "font-medium"}`}
            >
              {session.title}
            </span>
            <span className="mono flex-none text-label text-dim">
              <Ago at={session.lastEventAt} />
            </span>
          </span>
          {(projectName || session.branch || session.pr) && (
            <span className="flex w-full items-center gap-2 pl-4">
              {projectName && (
                <span className="max-w-[50%] flex-none truncate text-label text-dim">
                  {projectName}
                </span>
              )}
              <span className="mono min-w-0 flex-1 truncate text-label text-dim">
                {session.branch}
              </span>
              {session.pr && <PRMark pr={session.pr} />}
            </span>
          )}
        </button>
        {ended && (
          <button
            className="btn btn-ghost mr-1 h-6 w-6 flex-none justify-center px-0 opacity-0 group-focus-within:opacity-100 group-hover:opacity-100"
            title="Dismiss"
            aria-label={`Dismiss ${session.title}`}
            onClick={() => report(() => dismissSession(session.id))}
          >
            <Icon name="close" size={12} />
          </button>
        )}
      </div>
      {session.cleanup !== "" && (
        <div className="flex w-full pr-2 pb-1.5 pl-7">
          <CleanupControls session={session} />
        </div>
      )}
    </div>
  );
}

export const ThreadRow = memo(ThreadRowView);
