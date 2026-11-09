const API = "http://localhost:8080";

class ApiError extends Error {
    constructor(status, code, message) {
        super(message);
        this.name = "ApiError";
        this.status = status;
        this.code = code;
    }
}

// Every call goes through here: your Go decodeJSON + writeInternal idea.
async function request(path, init) {
    let res;
    try {
        res = await fetch(API + path, init);
    } catch {
        throw new ApiError(0, "network_error", "Cannot reach the server");   // server down
    }
    if (!res.ok) {
        const body = await res.json().catch(() => null);     // not JSON? null instead of crashing
        throw new ApiError(res.status, body?.error?.code ?? "unknown", body?.error?.message ?? `HTTP ${res.status}`);
    }
    return res.json();
}

const shortUrl = (code) => `${API}/${code}`;
const createLink = (url) =>
    request("/v1/links", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ url }) });

function listLinks({ limit = 20, cursor } = {}) {
    const params = new URLSearchParams({ limit: String(limit) });   // builds and escapes ?limit=20
    if (cursor) params.set("cursor", cursor);
    return request(`/v1/links?${params}`);
}

try {
    const link = await createLink("https://example.com/hello");
    console.log("created:", shortUrl(link.short_code));
    const page = await listLinks({ limit: 3 });
    console.log("codes:", page.items.map((l) => l.short_code));
    console.log("clicks:", page.items.reduce((s, l) => s + l.click_count, 0));
    if (page.next_cursor) {
        const page2 = await listLinks({ limit: 3, cursor: page.next_cursor });   // your Step 1 keyset, from JS
        console.log("page 2:", page2.items.map((l) => l.short_code));
    }
    await createLink("ftp://not-allowed.com");   // your backend answers 422
} catch (e) {
    if (e instanceof ApiError) console.log("API error ->", e.status, e.code, "|", e.message);
    else throw e;
}