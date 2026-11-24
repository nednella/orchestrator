import type { ReactNode } from "react";
import { useLayout } from "../LayoutContext";
import { Icon } from "./Icon";

type DigestHeaderProps = {
  updated: string;
  running: boolean;
  onRun(): void;
  children: ReactNode;
};

export function DigestHeader({
  updated,
  running,
  onRun,
  children,
}: DigestHeaderProps) {
  const { closeCentre } = useLayout();

  return (
    <>
      <header className="panel-head flex h-10 flex-none items-center gap-2 border-b border-line px-4">
        <h2 className="truncate text-body font-semibold">Weekly digest</h2>
        <span className="mono ml-auto flex-none text-small text-dim">
          {updated}
        </span>
        <button
          className="btn h-7 w-7 flex-none justify-center px-0"
          title="Run now"
          aria-label="Run now"
          disabled={running}
          onClick={onRun}
        >
          <span
            className={running ? "inline-flex animate-spin" : "inline-flex"}
          >
            <Icon name="refresh" size={13} />
          </span>
        </button>
        <button
          className="btn btn-ghost h-7 w-7 flex-none justify-center px-0"
          title="Close (Esc)"
          aria-label="Close the digest"
          onClick={closeCentre}
        >
          <Icon name="close" />
        </button>
      </header>
      <p className="flex-none border-b border-line px-6 py-2.5 text-small text-dim">
        {children}
      </p>
    </>
  );
}
