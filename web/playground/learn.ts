
type RefreshState = "idle" | "loading" | "failed";     // union of literals, like a Kotlin enum


function show(refreshState: RefreshState, cursor: string | null = null): void {
    if (cursor) {
        console.log(`Cursor: ${cursor}`);
    } else {
        console.log("No cursor available.");
    }

    switch (refreshState) {
        case "idle":
            console.log("Refresh state: idle");
            break;
        case "loading":
            console.log("Refresh state: loading");
            break;
        case "failed":
            console.log("Refresh state: failed");
            break;
        default: // This will cause a compile-time error if refreshState is not one of the defined types
            throw new Error(`Unhandled refresh state: ${refreshState}`);
    }
}



function main(): void {
    console.log("Starting application...");
    const s: RefreshState = "loading";                          // ERROR: not one of the three

    show(s)

    // Start your application
}

main();
