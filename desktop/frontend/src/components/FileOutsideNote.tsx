type FileOutsideNoteProps = { path: string }

// FileOutsideNote says that the open file is outside the thread's folder, so that it is read only, and where it is.
export function FileOutsideNote({ path }: FileOutsideNoteProps) {
  return <p className="flex-none border-b border-(--border-rule) bg-(--bg-code) px-3 py-1.5 text-label break-all text-soft">Read only, outside this thread&apos;s folder: {path}</p>
}
