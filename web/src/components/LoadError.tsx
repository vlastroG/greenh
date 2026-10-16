// shown when a page's first load fails; polling keeps retrying in the background
export function LoadError({ what }: { what: string }) {
  return <p className="text-sm text-amber-700">{`Couldn't load ${what} — retrying`}</p>
}
