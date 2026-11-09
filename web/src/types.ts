export interface Link {
    short_code: string;
    long_url: string;
    created_at: string;       // ISO date text
    click_count: number;
}

export interface LinkPage {
    items: Link[];
    next_cursor: string | null;   // null on the last page
}

export interface ErrorBody {
    error: { status: number; code: string; message: string; request_id?: string };
}