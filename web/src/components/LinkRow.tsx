import type { Link } from "../types";

export default function LinkRow({ link }: { link: Link }) {
  return <li>{link.short_code} · {link.click_count} clicks</li>;
}