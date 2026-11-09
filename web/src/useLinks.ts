import { useEffect, useState } from "react";
import { listLinks, messageOf } from "./api";
import type { Link } from "./types";

export function useLinks() {
    const [links, setLinks] = useState<Link[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        let cancelled = false;
        listLinks({ limit: 10 })
            .then((page) => { if (!cancelled) setLinks(page.items); })
            .catch((e) => { if (!cancelled) setError(messageOf(e)); })
            .finally(() => { if (!cancelled) setLoading(false); });
        return () => { cancelled = true; };
    }, []);

    return { links, loading, error };
}