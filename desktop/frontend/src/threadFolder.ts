// threadFolder is how a thread's folder reads: the project folder, a path inside the project relative to it, or any other
// path in full. A thread with no folder of its own runs in the project folder.
export function threadFolder(folder: string, projectDir: string): string {
  if (folder === '' || folder === projectDir) return 'Project folder'
  const prefix = projectDir.endsWith('/') ? projectDir : `${projectDir}/`
  return projectDir !== '' && folder.startsWith(prefix) ? folder.slice(prefix.length) : folder
}
