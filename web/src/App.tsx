import LinkRow from "./components/LinkRow";
import { useLinks } from "./useLinks";

export default function App() {
  const { links, loading, error } = useLinks();
  return (
    <main>
      <h1>LinkShrink</h1>
      {error && <p className="error">{error}</p>}
      {loading ? <p>Loading...</p> : <ul>{links.map((l) => <LinkRow key={l.short_code} link={l} />)}</ul>}
    </main>
  );
}