(function () {
    "use strict";

    var STORAGE_CHATS = "flag-editor-chats";
    var STORAGE_ACTIVE = "flag-editor-active-chat";
    var STORAGE_LEGACY_HISTORY = "flag-editor-chat-history";
    var STORAGE_MODEL = "flag-editor-model";
    var STORAGE_ACTIVE_STREAMS = "flag-editor-active-streams";
    var MCP_DOCS_URL =
        "https://github.com/zackarysantana/mongo-openfeature-go#mcp-server";
    var SCROLL_BOTTOM_THRESHOLD = 80;
    var TITLE_MAX = 40;
    var DEFAULT_MODEL = "openai/gpt-4o-mini";
    var MODEL_OPTION_LIMIT = 200;

    var chatOverlay = document.getElementById("chat-overlay");
    var chatToggle = document.getElementById("chat-toggle");
    var chatClose = document.getElementById("chat-close");
    var chatMessages = document.getElementById("chat-messages");
    var chatForm = document.getElementById("chat-form");
    var chatConnect = document.getElementById("chat-connect");
    var chatInput = document.getElementById("chat-input");
    var chatStatus = document.getElementById("chat-status");
    var chatList = document.getElementById("chat-list");
    var chatNewBtn = document.getElementById("chat-new-btn");
    var authLoginBtn = document.getElementById("auth-login-btn");
    var authLogoutBtn = document.getElementById("auth-logout-btn");
    var chatShortcutHint = document.getElementById("chat-shortcut-hint");
    var themeShortcutHint = document.getElementById("theme-shortcut-hint");
    var themeToggle = document.getElementById("theme-toggle");
    var modelPicker = document.getElementById("model-picker");
    var modelTrigger = document.getElementById("model-picker-trigger");
    var modelLabel = document.getElementById("model-picker-label");
    var modelMenu = document.getElementById("model-picker-menu");
    var modelSearch = document.getElementById("model-picker-search");
    var modelList = document.getElementById("model-picker-list");
    var modelStatus = document.getElementById("model-picker-status");
    var modelChips = document.getElementById("model-picker-chips");
    var modelSortBtn = document.getElementById("model-picker-sort");
    var modelSortLabel = document.getElementById("model-picker-sort-label");

    function modelTotalCost(m) {
        var p = m.pricing || {};
        var prompt = parseFloat(p.prompt);
        var completion = parseFloat(p.completion);
        if (isNaN(prompt) || isNaN(completion)) return null;
        return prompt + completion;
    }

    function compareNullable(a, b, desc) {
        var aMissing = a == null;
        var bMissing = b == null;
        if (aMissing && bMissing) return 0;
        if (aMissing) return 1; // missing values always last
        if (bMissing) return -1;
        return desc ? b - a : a - b;
    }

    var SORT_MODES = [
        { id: "default", label: "A–Z", comparator: null },
        {
            id: "cost-asc",
            label: "Cost ↑",
            comparator: function (a, b) {
                return compareNullable(
                    modelTotalCost(a),
                    modelTotalCost(b),
                    false,
                );
            },
        },
        {
            id: "cost-desc",
            label: "Cost ↓",
            comparator: function (a, b) {
                return compareNullable(
                    modelTotalCost(a),
                    modelTotalCost(b),
                    true,
                );
            },
        },
        {
            id: "context-asc",
            label: "Context ↑",
            comparator: function (a, b) {
                return compareNullable(
                    a.context_length,
                    b.context_length,
                    false,
                );
            },
        },
        {
            id: "context-desc",
            label: "Context ↓",
            comparator: function (a, b) {
                return compareNullable(
                    a.context_length,
                    b.context_length,
                    true,
                );
            },
        },
    ];
    var activeSortIndex = 0;

    var QUICK_FILTERS = [
        {
            id: "free",
            label: "Free",
            test: function (m) {
                var p = m.pricing || {};
                var prompt = parseFloat(p.prompt);
                var completion = parseFloat(p.completion);
                if (isNaN(prompt) || isNaN(completion)) return false;
                return prompt === 0 && completion === 0;
            },
        },
        {
            id: "anthropic",
            label: "Anthropic",
            test: function (m) {
                return /^anthropic\//i.test(m.id);
            },
        },
        {
            id: "openai",
            label: "OpenAI",
            test: function (m) {
                return /^openai\//i.test(m.id);
            },
        },
        {
            id: "google",
            label: "Google",
            test: function (m) {
                return /^google\//i.test(m.id);
            },
        },
    ];

    var activeQuickFilter = null;

    var chats = [];
    var activeChatId = null;
    var connected = false;
    var busy = false;
    var stickToBottom = true;
    // chatId -> { streamId, eventSource, state } for every assistant turn
    // currently in flight. Streams keep running on the server even when the
    // sidebar is closed or the user navigates to a new page; this map is
    // re-hydrated on boot from localStorage so reconnect is automatic.
    var activeStreams = {};
    var pendingPosts = {};

    var selectedModel = loadSelectedModel();
    var modelsCache = null;
    var modelsLoading = null;
    var modelsError = null;

    /* ============================================================
       Chat store
       ============================================================ */

    function makeChatId() {
        return (
            "c_" +
            Date.now().toString(36) +
            "_" +
            Math.random().toString(36).slice(2, 7)
        );
    }

    function createChat() {
        return {
            id: makeChatId(),
            title: "",
            history: [],
            createdAt: Date.now(),
            updatedAt: Date.now(),
        };
    }

    function loadChats() {
        var raw = null;
        try {
            raw = localStorage.getItem(STORAGE_CHATS);
        } catch (_e) {}

        if (raw) {
            try {
                var parsed = JSON.parse(raw);
                if (Array.isArray(parsed) && parsed.length > 0) {
                    chats = parsed.filter(function (c) {
                        return c && typeof c === "object" && c.id;
                    });
                    if (chats.length > 0) {
                        try {
                            activeChatId =
                                localStorage.getItem(STORAGE_ACTIVE) || null;
                        } catch (_e) {}
                        if (!activeChatId || !findChat(activeChatId)) {
                            activeChatId = chats[0].id;
                        }
                        return;
                    }
                }
            } catch (_e) {}
        }

        // Migrate legacy single-history storage
        var legacyRaw = null;
        try {
            legacyRaw = localStorage.getItem(STORAGE_LEGACY_HISTORY);
        } catch (_e) {}
        if (legacyRaw) {
            try {
                var legacy = JSON.parse(legacyRaw);
                if (Array.isArray(legacy) && legacy.length > 0) {
                    var migrated = createChat();
                    migrated.history = legacy;
                    migrated.title = deriveTitle(legacy);
                    migrated.updatedAt = Date.now();
                    chats = [migrated];
                    activeChatId = migrated.id;
                    saveChats();
                    try {
                        localStorage.removeItem(STORAGE_LEGACY_HISTORY);
                    } catch (_e) {}
                    return;
                }
            } catch (_e) {}
        }

        var first = createChat();
        chats = [first];
        activeChatId = first.id;
        saveChats();
    }

    function saveChats() {
        try {
            localStorage.setItem(STORAGE_CHATS, JSON.stringify(chats));
            localStorage.setItem(STORAGE_ACTIVE, activeChatId || "");
        } catch (_e) {}
    }

    function findChat(id) {
        for (var i = 0; i < chats.length; i++) {
            if (chats[i].id === id) return chats[i];
        }
        return null;
    }

    function activeChat() {
        return findChat(activeChatId);
    }

    function deriveTitle(history) {
        for (var i = 0; i < history.length; i++) {
            var m = history[i];
            if (m && m.role === "user" && m.content) {
                var t = String(m.content).trim().replace(/\s+/g, " ");
                return t.length > TITLE_MAX
                    ? t.slice(0, TITLE_MAX - 1) + "…"
                    : t;
            }
        }
        return "";
    }

    function ensureTitle(chat) {
        if (!chat || chat.title) return;
        chat.title = deriveTitle(chat.history);
    }

    function selectChat(id, opts) {
        if (id === activeChatId) return;
        var target = findChat(id);
        if (!target) return;
        activeChatId = id;
        saveChats();
        renderHistory();
        renderChatList();
        // If the new active chat has an in-flight stream, re-attach so the
        // partial DOM is rebuilt from the server's buffered events.
        var entry = activeStreams[id];
        if (entry && entry.streamId) {
            attachStream(target, entry.streamId);
        }
        refreshBusy();
        if (!opts || opts.focusInput !== false) focusChatInput();
    }

    function newChat() {
        // If the current chat is empty, just keep using it.
        var current = activeChat();
        if (
            current &&
            current.history.length === 0 &&
            !activeStreams[current.id]
        ) {
            renderHistory();
            renderChatList();
            focusChatInput();
            return;
        }
        var c = createChat();
        chats.unshift(c);
        activeChatId = c.id;
        saveChats();
        renderHistory();
        renderChatList();
        refreshBusy();
        focusChatInput();
    }

    function deleteChat(id) {
        var idx = -1;
        for (var i = 0; i < chats.length; i++) {
            if (chats[i].id === id) {
                idx = i;
                break;
            }
        }
        if (idx === -1) return;
        if (activeStreams[id]) {
            closeStream(id);
        }
        chats.splice(idx, 1);
        if (chats.length === 0) {
            var fresh = createChat();
            chats.push(fresh);
            activeChatId = fresh.id;
        } else if (id === activeChatId) {
            activeChatId = chats[Math.max(0, idx - 1)].id;
        }
        saveChats();
        renderHistory();
        renderChatList();
        refreshBusy();
    }

    /* ============================================================
       Chat list rendering
       ============================================================ */

    function renderChatList() {
        if (!chatList) return;
        chatList.innerHTML = "";
        var sorted = chats.slice().sort(function (a, b) {
            return (b.updatedAt || 0) - (a.updatedAt || 0);
        });

        sorted.forEach(function (c) {
            chatList.appendChild(buildChatListItem(c));
        });
    }

    function buildChatListItem(chat) {
        var item = document.createElement("button");
        item.type = "button";
        item.className = "chat-list__item";
        item.setAttribute("role", "listitem");
        if (chat.id === activeChatId) item.classList.add("is-active");
        if (chat.id in activeStreams || chat.id in pendingPosts) {
            item.classList.add("is-streaming");
        }
        item.dataset.chatId = chat.id;

        var title = document.createElement("div");
        title.className = "chat-list__title";
        var displayTitle = chat.title || deriveTitle(chat.history);
        if (displayTitle) {
            title.textContent = displayTitle;
        } else {
            title.classList.add("is-empty");
            title.textContent = "New chat";
        }
        item.appendChild(title);

        var meta = document.createElement("div");
        meta.className = "chat-list__meta";
        meta.textContent = chatMetaLabel(chat);
        item.appendChild(meta);

        var del = document.createElement("button");
        del.type = "button";
        del.className = "chat-list__delete";
        del.setAttribute("aria-label", "Delete chat");
        del.title = "Delete chat";
        del.innerHTML =
            '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/><path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>';
        del.addEventListener("click", function (e) {
            e.stopPropagation();
            if (chats.length > 1 || (chat.history && chat.history.length > 0)) {
                if (!window.confirm("Delete this chat?")) return;
            }
            deleteChat(chat.id);
        });
        item.appendChild(del);

        item.addEventListener("click", function () {
            selectChat(chat.id);
        });

        return item;
    }

    function chatMetaLabel(chat) {
        var msgs = (chat.history || []).filter(function (m) {
            return m.role === "user" || m.role === "assistant";
        }).length;
        var when = chat.updatedAt ? formatRelativeTime(chat.updatedAt) : "";
        if (msgs === 0) return when || "empty";
        var parts = [msgs + (msgs === 1 ? " msg" : " msgs")];
        if (when) parts.push(when);
        return parts.join(" · ");
    }

    function formatRelativeTime(ts) {
        var diff = Date.now() - ts;
        if (diff < 0) return "now";
        var sec = Math.floor(diff / 1000);
        if (sec < 60) return "now";
        var min = Math.floor(sec / 60);
        if (min < 60) return min + "m";
        var hr = Math.floor(min / 60);
        if (hr < 24) return hr + "h";
        var day = Math.floor(hr / 24);
        if (day < 7) return day + "d";
        var wk = Math.floor(day / 7);
        if (wk < 5) return wk + "w";
        var mo = Math.floor(day / 30);
        if (mo < 12) return mo + "mo";
        return Math.floor(day / 365) + "y";
    }

    /* ============================================================
       Markdown
       ============================================================ */

    function renderMarkdown(text) {
        if (!text) return "";
        if (typeof window.marked !== "undefined") {
            try {
                return window.marked.parse(text, {
                    gfm: true,
                    breaks: true,
                    headerIds: false,
                    mangle: false,
                });
            } catch (_e) {
                return escapeHtml(text);
            }
        }
        return escapeHtml(text).replace(/\n/g, "<br>");
    }

    function escapeHtml(s) {
        return String(s)
            .replace(/&/g, "&amp;")
            .replace(/</g, "&lt;")
            .replace(/>/g, "&gt;")
            .replace(/"/g, "&quot;");
    }

    /* ============================================================
       Scroll handling
       ============================================================ */

    function isNearBottom() {
        if (!chatMessages) return true;
        var distance =
            chatMessages.scrollHeight -
            chatMessages.scrollTop -
            chatMessages.clientHeight;
        return distance <= SCROLL_BOTTOM_THRESHOLD;
    }

    function scrollToBottom(immediate) {
        if (!chatMessages) return;
        if (immediate) {
            chatMessages.scrollTop = chatMessages.scrollHeight;
        } else {
            requestAnimationFrame(function () {
                chatMessages.scrollTop = chatMessages.scrollHeight;
            });
        }
    }

    function maybeScroll() {
        if (stickToBottom) scrollToBottom(false);
    }

    function wireScrollTracking() {
        if (!chatMessages) return;
        chatMessages.addEventListener("scroll", function () {
            stickToBottom = isNearBottom();
        });
    }

    /* ============================================================
       Rendering history (static)
       ============================================================ */

    function renderHistory() {
        if (!chatMessages) return;
        chatMessages.innerHTML = "";

        var hist = (activeChat() && activeChat().history) || [];

        if (hist.length === 0) {
            chatMessages.appendChild(buildEmptyState());
            scrollToBottom(true);
            stickToBottom = true;
            return;
        }

        var i = 0;
        while (i < hist.length) {
            var msg = hist[i];

            if (msg.role === "user") {
                chatMessages.appendChild(renderUserBubble(msg.content));
                i++;
                continue;
            }

            if (msg.role === "error") {
                chatMessages.appendChild(renderErrorBubble(msg.content));
                i++;
                continue;
            }

            if (msg.role === "turn_cost") {
                chatMessages.appendChild(
                    renderCostFooter(msg.cost, msg.tokens),
                );
                i++;
                continue;
            }

            if (msg.role === "assistant") {
                if (msg.tool_calls && msg.tool_calls.length) {
                    var thinking = buildThinkingBlock();
                    msg.tool_calls.forEach(function (tc) {
                        var name =
                            tc.function && tc.function.name
                                ? tc.function.name
                                : "tool";
                        var args =
                            tc.function && tc.function.arguments
                                ? tc.function.arguments
                                : "";
                        var item = buildThinkingItem(name, args, true);
                        thinking.list.appendChild(item.el);
                        thinking.count++;
                    });
                    chatMessages.appendChild(thinking.el);
                    i++;

                    while (i < hist.length && hist[i].role === "tool") {
                        i++;
                    }
                    updateThinkingSummary(thinking);
                    continue;
                }
                if (msg.content) {
                    chatMessages.appendChild(
                        renderAssistantBubble(msg.content),
                    );
                }
                i++;
                continue;
            }

            i++;
        }

        scrollToBottom(true);
        stickToBottom = true;
    }

    function buildEmptyState() {
        var empty = document.createElement("div");
        empty.className = "chat-empty";
        empty.innerHTML =
            '<div class="chat-mcp-note">' +
            "Prefer using an external AI agent? Connect via the " +
            '<a href="' +
            MCP_DOCS_URL +
            '" target="_blank" rel="noopener noreferrer">MCP server</a> ' +
            "documented in the README for the same flag operations from Cursor, Claude Desktop, and other MCP clients." +
            "</div>";
        return empty;
    }

    function renderUserBubble(text) {
        var el = document.createElement("div");
        el.className = "chat-msg chat-msg--user";
        el.textContent = text || "";
        return el;
    }

    function renderAssistantBubble(text) {
        var el = document.createElement("div");
        el.className = "chat-msg chat-msg--assistant";
        var content = document.createElement("div");
        content.className = "chat-msg__content markdown";
        content.innerHTML = renderMarkdown(text);
        el.appendChild(content);
        return el;
    }

    function renderErrorBubble(text) {
        var el = document.createElement("div");
        el.className = "chat-msg chat-msg--error";
        el.textContent = text || "Something went wrong.";
        return el;
    }

    function formatCost(cost) {
        if (cost == null || isNaN(cost)) return "";
        var n = Number(cost);
        if (n === 0) return "$0.00";
        if (n < 0.0001) return "$" + n.toFixed(6);
        if (n < 0.01) return "$" + n.toFixed(4);
        return "$" + n.toFixed(3);
    }

    function renderCostFooter(cost, tokens) {
        var el = document.createElement("div");
        el.className = "chat-cost";
        var parts = [];
        var formatted = formatCost(cost);
        if (formatted) parts.push(formatted);
        if (tokens != null && tokens > 0) {
            parts.push(tokens.toLocaleString() + " tokens");
        }
        el.textContent = parts.join(" · ");
        return el;
    }

    function ensureCostFooter(state) {
        if (!state.costFooter) {
            state.costFooter = renderCostFooter(null, null);
            state.costFooter.hidden = true;
        }
        // Always re-pin to the bottom in case more content was appended
        // after a previous render (e.g. a follow-up round after tool calls).
        if (state.costFooter.parentNode !== chatMessages) {
            chatMessages.appendChild(state.costFooter);
        } else if (chatMessages.lastChild !== state.costFooter) {
            chatMessages.appendChild(state.costFooter);
        }
        return state.costFooter;
    }

    function updateCostFooter(state) {
        if (!isStreamForActive(state)) return;
        var footer = ensureCostFooter(state);
        if (state.turnCost == null && !state.turnTokens) {
            footer.hidden = true;
            return;
        }
        footer.hidden = false;
        var parts = [];
        var formatted = formatCost(state.turnCost);
        if (formatted) parts.push(formatted);
        if (state.turnTokens > 0) {
            parts.push(state.turnTokens.toLocaleString() + " tokens");
        }
        footer.textContent = parts.join(" · ");
        maybeScroll();
    }

    /* ============================================================
       Thinking (collapsible tool-call group)
       ============================================================ */

    function buildThinkingBlock() {
        var details = document.createElement("details");
        details.className = "chat-thinking";

        var summary = document.createElement("summary");
        summary.className = "chat-thinking__summary";

        var caret = document.createElement("span");
        caret.className = "chat-thinking__caret";
        caret.innerHTML =
            '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="9 18 15 12 9 6"/></svg>';

        var label = document.createElement("span");
        label.className = "chat-thinking__label";
        label.textContent = "Thinking";

        summary.appendChild(caret);
        summary.appendChild(label);

        var list = document.createElement("ul");
        list.className = "chat-thinking__list";

        details.appendChild(summary);
        details.appendChild(list);

        return {
            el: details,
            summary: summary,
            label: label,
            list: list,
            count: 0,
            running: 0,
        };
    }

    function buildThinkingItem(name, argsJson, completed) {
        var li = document.createElement("li");
        li.className = "chat-thinking__item";
        if (completed) li.classList.add("is-done");

        var status = document.createElement("span");
        status.className = "chat-thinking__status";
        if (!completed) {
            status.classList.add("is-running");
        }

        var nameEl = document.createElement("code");
        nameEl.className = "chat-thinking__name";
        nameEl.textContent = name || "tool";

        li.appendChild(status);
        li.appendChild(nameEl);

        if (argsJson) {
            var formatted = formatToolArgs(argsJson);
            if (formatted.short) {
                var argsEl = document.createElement("span");
                argsEl.className = "chat-thinking__args";
                argsEl.textContent = formatted.short;
                li.appendChild(argsEl);
            }
            if (formatted.full) {
                li.title = (name || "tool") + "  " + formatted.full;
            }
        }

        return { el: li, status: status };
    }

    function formatToolArgs(argsJson) {
        var empty = { short: "", full: "" };
        if (!argsJson) return empty;
        try {
            var parsed = JSON.parse(argsJson);
            if (!parsed || typeof parsed !== "object") return empty;
            var keys = Object.keys(parsed);
            if (keys.length === 0) return empty;
            var shortParts = [];
            var fullParts = [];
            keys.forEach(function (k) {
                var v = parsed[k];
                if (typeof v === "string") {
                    var trimmed = v;
                    if (trimmed.length > 40) {
                        trimmed = trimmed.slice(0, 37) + "…";
                    }
                    shortParts.push(k + "=" + JSON.stringify(trimmed));
                    fullParts.push(k + "=" + JSON.stringify(v));
                } else if (typeof v === "object") {
                    shortParts.push(k + "=…");
                    fullParts.push(k + "=" + JSON.stringify(v));
                } else {
                    var s = k + "=" + String(v);
                    shortParts.push(s);
                    fullParts.push(s);
                }
            });
            return {
                short: shortParts.join(" "),
                full: fullParts.join(" "),
            };
        } catch (_e) {
            return empty;
        }
    }

    function updateThinkingSummary(thinking) {
        if (!thinking) return;
        var n = thinking.count;
        if (thinking.running > 0) {
            thinking.label.textContent =
                "Thinking… " + thinking.running + " running";
            thinking.el.classList.add("is-running");
        } else {
            thinking.label.textContent =
                n === 1 ? "Used 1 tool" : "Used " + n + " tools";
            thinking.el.classList.remove("is-running");
        }
    }

    /* ============================================================
       Streaming
       ============================================================ */

    function ensureNotEmpty() {
        var empty = chatMessages.querySelector(".chat-empty");
        if (empty) empty.remove();
    }

    function createStreamingState(chatId, streamId) {
        return {
            chatId: chatId,
            streamId: streamId || null,
            thinking: null,
            assistantEl: null,
            assistantContent: null,
            assistantText: "",
            toolItems: {},
            turnCost: null,
            turnTokens: 0,
            costFooter: null,
        };
    }

    function isStreamForActive(state) {
        return state && state.chatId === activeChatId;
    }

    function startThinking(state) {
        if (!isStreamForActive(state)) return;
        if (!state.thinking) {
            state.thinking = buildThinkingBlock();
            chatMessages.appendChild(state.thinking.el);
            maybeScroll();
        }
        state.assistantEl = null;
        state.assistantContent = null;
        state.assistantText = "";
    }

    function startAssistantBubble(state) {
        if (!isStreamForActive(state)) return;
        if (state.assistantEl) return;
        state.assistantEl = document.createElement("div");
        state.assistantEl.className = "chat-msg chat-msg--assistant";
        state.assistantContent = document.createElement("div");
        state.assistantContent.className = "chat-msg__content markdown";
        state.assistantEl.appendChild(state.assistantContent);
        chatMessages.appendChild(state.assistantEl);
    }

    function appendDelta(state, content) {
        if (!content) return;
        if (!isStreamForActive(state)) return;
        if (!state.assistantEl) startAssistantBubble(state);
        state.assistantText += content;
        state.assistantContent.innerHTML = renderMarkdown(state.assistantText);
        maybeScroll();
    }

    function handleToolCall(state, evt) {
        if (!isStreamForActive(state)) return;
        if (!state.thinking) startThinking(state);
        var item = buildThinkingItem(evt.name, evt.arguments, false);
        state.thinking.list.appendChild(item.el);
        state.thinking.count++;
        state.thinking.running++;
        state.toolItems[evt.id] = item;
        updateThinkingSummary(state.thinking);
        maybeScroll();
    }

    function handleToolResult(state, evt) {
        if (!isStreamForActive(state)) return;
        var item = state.toolItems[evt.id];
        if (!item || !state.thinking) return;
        item.el.classList.add("is-done");
        if (evt.ok === false) {
            item.el.classList.add("is-error");
            if (evt.error) item.el.title = evt.error;
        }
        item.status.classList.remove("is-running");
        state.thinking.running = Math.max(0, state.thinking.running - 1);
        updateThinkingSummary(state.thinking);
    }

    function handleStreamError(state, message) {
        if (!isStreamForActive(state)) return;
        if (state && state.thinking) {
            state.thinking.running = 0;
            updateThinkingSummary(state.thinking);
        }
        chatMessages.appendChild(renderErrorBubble(message));
        maybeScroll();
    }

    /* ============================================================
       Active streams (server-buffered, reconnectable)
       ============================================================ */

    function loadStoredActiveStreams() {
        try {
            var raw = localStorage.getItem(STORAGE_ACTIVE_STREAMS);
            if (raw) {
                var parsed = JSON.parse(raw);
                if (parsed && typeof parsed === "object") return parsed;
            }
        } catch (_e) {}
        return {};
    }

    function persistActiveStreams() {
        var snapshot = {};
        Object.keys(activeStreams).forEach(function (chatId) {
            snapshot[chatId] = { streamId: activeStreams[chatId].streamId };
        });
        try {
            localStorage.setItem(
                STORAGE_ACTIVE_STREAMS,
                JSON.stringify(snapshot),
            );
        } catch (_e) {}
    }

    function attachStream(chat, streamId) {
        if (!chat || !streamId) return;
        // Always close any existing subscription for this chat first; the new
        // EventSource will replay all events from the server's buffer so the
        // DOM is rebuilt cleanly when reattaching after a chat switch or page
        // navigation.
        var existing = activeStreams[chat.id];
        if (existing && existing.eventSource) {
            try {
                existing.eventSource.close();
            } catch (_e) {}
        }

        var state = createStreamingState(chat.id, streamId);
        var url = "/api/chat/stream?id=" + encodeURIComponent(streamId);

        var es;
        try {
            es = new EventSource(url, { withCredentials: true });
        } catch (_e) {
            chat.history.push({
                role: "error",
                content: "Could not start assistant stream.",
            });
            chat.updatedAt = Date.now();
            saveChats();
            renderChatList();
            return;
        }

        activeStreams[chat.id] = {
            streamId: streamId,
            eventSource: es,
            state: state,
        };
        persistActiveStreams();

        if (chat.id === activeChatId) {
            ensureNotEmpty();
        }

        es.onmessage = function (e) {
            var evt;
            try {
                evt = JSON.parse(e.data);
            } catch (_err) {
                return;
            }
            handleStreamEvent(chat, state, evt);
        };

        es.onerror = function () {
            // EventSource auto-retries on transient network errors. A CLOSED
            // ready state means the server returned a non-200 (commonly 404
            // because the buffered session has expired).
            if (es.readyState === EventSource.CLOSED) {
                handleStreamGone(chat, state);
            }
        };

        refreshBusy();
        renderChatList();
    }

    function handleStreamEvent(chat, state, evt) {
        var render = state.chatId === activeChatId;
        switch (evt.type) {
            case "message_start":
                break;
            case "delta":
                if (render) appendDelta(state, evt.content || "");
                break;
            case "tool_call":
                if (render) handleToolCall(state, evt);
                break;
            case "tool_result":
                if (render) handleToolResult(state, evt);
                break;
            case "usage":
                if (evt.cost != null) {
                    state.turnCost = (state.turnCost || 0) + evt.cost;
                }
                if (evt.tokens) {
                    state.turnTokens += evt.tokens;
                }
                break;
            case "message_end":
                if (render) {
                    state.assistantEl = null;
                    state.assistantContent = null;
                    state.assistantText = "";
                }
                break;
            case "error":
                finalizeStreamError(
                    chat,
                    state,
                    evt.error || "Assistant error.",
                    render,
                );
                break;
            case "done":
                finalizeStreamSuccess(chat, state, evt, render);
                break;
        }
    }

    function finalizeStreamSuccess(chat, state, evt, render) {
        if (chat.lastDoneStreamId === state.streamId) {
            // Already saved (e.g. reconnected to a session we'd already
            // finalized). Just disconnect.
            closeStream(chat.id);
            return;
        }
        chat.lastDoneStreamId = state.streamId;
        if (evt.cost != null) state.turnCost = evt.cost;
        if (evt.tokens) state.turnTokens = evt.tokens;

        var serverMessages = evt.messages || [];
        for (var i = 0; i < serverMessages.length; i++) {
            chat.history.push(serverMessages[i]);
        }
        if (state.turnCost != null || state.turnTokens > 0) {
            chat.history.push({
                role: "turn_cost",
                cost: state.turnCost,
                tokens: state.turnTokens || null,
            });
        }
        chat.updatedAt = Date.now();
        ensureTitle(chat);
        saveChats();

        if (render) updateCostFooter(state);
        closeStream(chat.id);
    }

    function finalizeStreamError(chat, state, message, render) {
        if (chat.lastDoneStreamId === state.streamId) {
            closeStream(chat.id);
            return;
        }
        chat.lastDoneStreamId = state.streamId;
        chat.history.push({ role: "error", content: message });
        chat.updatedAt = Date.now();
        saveChats();

        if (render) handleStreamError(state, message);
        closeStream(chat.id);
    }

    function handleStreamGone(chat, state) {
        if (chat.lastDoneStreamId === state.streamId) {
            closeStream(chat.id);
            return;
        }
        var msg = "Stream expired before it could finish.";
        chat.lastDoneStreamId = state.streamId;
        chat.history.push({ role: "error", content: msg });
        chat.updatedAt = Date.now();
        saveChats();
        if (state.chatId === activeChatId) {
            handleStreamError(state, msg);
        }
        closeStream(chat.id);
    }

    function closeStream(chatId) {
        var entry = activeStreams[chatId];
        if (!entry) return;
        try {
            entry.eventSource.close();
        } catch (_e) {}
        delete activeStreams[chatId];
        persistActiveStreams();
        refreshBusy();
        renderChatList();
    }

    function resumeActiveStreams() {
        var stored = loadStoredActiveStreams();
        Object.keys(stored).forEach(function (chatId) {
            var info = stored[chatId];
            if (!info || !info.streamId) return;
            var chat = findChat(chatId);
            if (!chat) return;
            attachStream(chat, info.streamId);
        });
    }

    /* ============================================================
       Model picker
       ============================================================ */

    function loadSelectedModel() {
        try {
            var stored = localStorage.getItem(STORAGE_MODEL);
            if (stored) return stored;
        } catch (_e) {}
        return DEFAULT_MODEL;
    }

    function saveSelectedModel() {
        try {
            localStorage.setItem(STORAGE_MODEL, selectedModel);
        } catch (_e) {}
    }

    function modelDisplayName(id) {
        if (!modelsCache) return id;
        for (var i = 0; i < modelsCache.length; i++) {
            if (modelsCache[i].id === id) {
                return modelsCache[i].name || modelsCache[i].id;
            }
        }
        return id;
    }

    function shortModelLabel(id) {
        if (!id) return "";
        var slash = id.indexOf("/");
        if (slash >= 0) return id.slice(slash + 1);
        return id;
    }

    function updateModelTriggerLabel() {
        if (!modelLabel) return;
        var name = modelsCache
            ? modelDisplayName(selectedModel)
            : shortModelLabel(selectedModel) || "Default model";
        modelLabel.textContent = name;
        if (modelTrigger) {
            modelTrigger.title = name + "\n" + selectedModel;
        }
    }

    function setModelStatus(message) {
        if (!modelStatus) return;
        if (!message) {
            modelStatus.hidden = true;
            modelStatus.textContent = "";
        } else {
            modelStatus.hidden = false;
            modelStatus.textContent = message;
        }
    }

    function formatModelMeta(model) {
        var parts = [];
        var pricing = model.pricing || {};
        var promptPrice = parseFloat(pricing.prompt);
        var completionPrice = parseFloat(pricing.completion);
        if (!isNaN(promptPrice) && !isNaN(completionPrice)) {
            if (promptPrice === 0 && completionPrice === 0) {
                parts.push("Free");
            } else {
                parts.push(
                    "$" +
                        formatPricePerMillion(promptPrice) +
                        " / $" +
                        formatPricePerMillion(completionPrice) +
                        " per M",
                );
            }
        }
        if (model.context_length) {
            parts.push(formatContext(model.context_length) + " ctx");
        }
        return parts.join(" · ");
    }

    function formatPricePerMillion(perToken) {
        var perM = perToken * 1000000;
        if (perM === 0) return "0";
        if (perM < 0.01) return perM.toFixed(4);
        if (perM < 1) return perM.toFixed(3);
        if (perM < 10) return perM.toFixed(2);
        return perM.toFixed(1);
    }

    function formatContext(n) {
        if (n >= 1000000)
            return (n / 1000000).toFixed(1).replace(/\.0$/, "") + "M";
        if (n >= 1000) return Math.round(n / 1000) + "K";
        return String(n);
    }

    function fetchModels() {
        if (modelsCache) return Promise.resolve(modelsCache);
        if (modelsLoading) return modelsLoading;
        modelsError = null;
        setModelStatus("Loading models…");
        modelsLoading = fetch("/api/models", { credentials: "same-origin" })
            .then(function (r) {
                if (!r.ok) {
                    return r.text().then(function (text) {
                        throw new Error(text || "Failed to load models");
                    });
                }
                return r.json();
            })
            .then(function (data) {
                var list = (data && data.data) || [];
                modelsCache = list
                    .filter(function (m) {
                        return m && m.id;
                    })
                    .sort(function (a, b) {
                        var an = (a.name || a.id).toLowerCase();
                        var bn = (b.name || b.id).toLowerCase();
                        if (an < bn) return -1;
                        if (an > bn) return 1;
                        return 0;
                    });
                setModelStatus("");
                updateModelTriggerLabel();
                return modelsCache;
            })
            .catch(function (err) {
                modelsError = err.message || "Failed to load models";
                setModelStatus(modelsError);
                throw err;
            })
            .finally(function () {
                modelsLoading = null;
            });
        return modelsLoading;
    }

    function filterModels(query) {
        if (!modelsCache) return [];
        var q = (query || "").trim().toLowerCase();
        var quick = null;
        if (activeQuickFilter) {
            for (var k = 0; k < QUICK_FILTERS.length; k++) {
                if (QUICK_FILTERS[k].id === activeQuickFilter) {
                    quick = QUICK_FILTERS[k];
                    break;
                }
            }
        }
        var matches = [];
        for (var i = 0; i < modelsCache.length; i++) {
            var m = modelsCache[i];
            if (quick && !quick.test(m)) continue;
            if (q) {
                var hay = (m.id + " " + (m.name || "")).toLowerCase();
                if (hay.indexOf(q) === -1) continue;
            }
            matches.push(m);
        }
        var sortMode = SORT_MODES[activeSortIndex];
        if (sortMode && sortMode.comparator) {
            matches.sort(sortMode.comparator);
        }
        if (matches.length > MODEL_OPTION_LIMIT) {
            matches.length = MODEL_OPTION_LIMIT;
        }
        return matches;
    }

    function updateSortButton() {
        if (!modelSortBtn || !modelSortLabel) return;
        var mode = SORT_MODES[activeSortIndex];
        modelSortLabel.textContent = mode.label;
        modelSortBtn.classList.toggle("is-custom", mode.id !== "default");
        modelSortBtn.title = "Sort: " + mode.label + " (click to change)";
    }

    function cycleSortMode() {
        activeSortIndex = (activeSortIndex + 1) % SORT_MODES.length;
        updateSortButton();
        renderModelOptions();
    }

    function renderModelChips() {
        if (!modelChips) return;
        if (modelChips.childNodes.length === QUICK_FILTERS.length) {
            // Already built; just sync active state.
            for (var j = 0; j < modelChips.children.length; j++) {
                var btn = modelChips.children[j];
                btn.classList.toggle(
                    "is-active",
                    btn.dataset.filterId === activeQuickFilter,
                );
                btn.setAttribute(
                    "aria-pressed",
                    btn.dataset.filterId === activeQuickFilter
                        ? "true"
                        : "false",
                );
            }
            return;
        }
        modelChips.innerHTML = "";
        QUICK_FILTERS.forEach(function (f) {
            var btn = document.createElement("button");
            btn.type = "button";
            btn.className = "model-picker__chip";
            btn.dataset.filterId = f.id;
            btn.textContent = f.label;
            btn.setAttribute(
                "aria-pressed",
                f.id === activeQuickFilter ? "true" : "false",
            );
            if (f.id === activeQuickFilter) btn.classList.add("is-active");
            btn.addEventListener("click", function (e) {
                e.preventDefault();
                activeQuickFilter = activeQuickFilter === f.id ? null : f.id;
                renderModelChips();
                renderModelOptions();
            });
            modelChips.appendChild(btn);
        });
    }

    function renderModelOptions() {
        if (!modelList) return;
        modelList.innerHTML = "";
        var query = modelSearch ? modelSearch.value : "";
        var items = filterModels(query);

        if (modelsError && (!modelsCache || modelsCache.length === 0)) {
            return;
        }
        if (!modelsCache) {
            return;
        }
        if (items.length === 0) {
            var empty = document.createElement("div");
            empty.className = "model-picker__status";
            empty.textContent = "No models match.";
            modelList.appendChild(empty);
            return;
        }

        var frag = document.createDocumentFragment();
        items.forEach(function (m) {
            var opt = document.createElement("button");
            opt.type = "button";
            opt.className = "model-picker__option";
            opt.setAttribute("role", "option");
            opt.dataset.modelId = m.id;
            if (m.id === selectedModel) {
                opt.classList.add("is-selected");
                opt.setAttribute("aria-selected", "true");
            }

            var name = document.createElement("span");
            name.className = "model-picker__option-name";
            name.textContent = m.name || m.id;
            opt.appendChild(name);

            var metaText = formatModelMeta(m);
            var metaParts = [m.id];
            if (metaText) metaParts.push(metaText);

            var meta = document.createElement("span");
            meta.className = "model-picker__option-meta";
            meta.textContent = metaParts.join(" — ");
            opt.appendChild(meta);

            opt.addEventListener("click", function () {
                selectModel(m.id);
            });

            frag.appendChild(opt);
        });
        modelList.appendChild(frag);
    }

    function selectModel(id) {
        if (!id || id === selectedModel) {
            closeModelPicker();
            return;
        }
        selectedModel = id;
        saveSelectedModel();
        updateModelTriggerLabel();
        closeModelPicker();
        focusChatInput();
    }

    function isModelPickerOpen() {
        return modelPicker && modelPicker.classList.contains("is-open");
    }

    function openModelPicker() {
        if (!modelPicker || !modelMenu) return;
        modelPicker.classList.add("is-open");
        modelMenu.hidden = false;
        if (modelTrigger) modelTrigger.setAttribute("aria-expanded", "true");

        if (modelSearch) {
            modelSearch.value = "";
            requestAnimationFrame(function () {
                modelSearch.focus();
            });
        }

        renderModelChips();
        updateSortButton();

        if (modelsCache) {
            renderModelOptions();
            scrollSelectedModelIntoView();
        } else {
            renderModelOptions();
            fetchModels()
                .then(function () {
                    if (isModelPickerOpen()) {
                        renderModelOptions();
                        scrollSelectedModelIntoView();
                    }
                })
                .catch(function () {});
        }
    }

    function scrollSelectedModelIntoView() {
        if (!modelList) return;
        var selected = modelList.querySelector(
            ".model-picker__option.is-selected",
        );
        if (selected && typeof selected.scrollIntoView === "function") {
            selected.scrollIntoView({ block: "nearest" });
        }
    }

    function closeModelPicker() {
        if (!modelPicker || !modelMenu) return;
        if (!modelPicker.classList.contains("is-open")) return;
        modelPicker.classList.remove("is-open");
        modelMenu.hidden = true;
        if (modelTrigger) modelTrigger.setAttribute("aria-expanded", "false");
    }

    function toggleModelPicker() {
        if (isModelPickerOpen()) {
            closeModelPicker();
        } else {
            openModelPicker();
        }
    }

    function wireModelPicker() {
        if (!modelPicker || !modelTrigger || !modelMenu) return;

        modelTrigger.addEventListener("click", function (e) {
            e.stopPropagation();
            toggleModelPicker();
        });

        if (modelSortBtn) {
            modelSortBtn.addEventListener("click", function (e) {
                e.preventDefault();
                e.stopPropagation();
                cycleSortMode();
            });
        }

        if (modelSearch) {
            modelSearch.addEventListener("input", function () {
                renderModelOptions();
            });
            modelSearch.addEventListener("keydown", function (e) {
                if (e.key === "Escape") {
                    e.preventDefault();
                    closeModelPicker();
                    if (modelTrigger) modelTrigger.focus();
                } else if (e.key === "Enter") {
                    e.preventDefault();
                    var first = modelList
                        ? modelList.querySelector(".model-picker__option")
                        : null;
                    if (first) first.click();
                }
            });
        }

        document.addEventListener("click", function (e) {
            if (!isModelPickerOpen()) return;
            if (modelPicker.contains(e.target)) return;
            closeModelPicker();
        });

        document.addEventListener("keydown", function (e) {
            if (e.key === "Escape" && isModelPickerOpen()) {
                closeModelPicker();
                if (modelTrigger) modelTrigger.focus();
            }
        });
    }

    /* ============================================================
       Send
       ============================================================ */

    function sendMessage(text) {
        if (!text.trim() || busy || !connected) return;
        var chat = activeChat();
        if (!chat) return;

        var trimmed = text.trim();
        var userMsg = { role: "user", content: trimmed };
        chat.history.push(userMsg);
        chat.updatedAt = Date.now();
        ensureTitle(chat);
        saveChats();
        renderChatList();

        ensureNotEmpty();
        chatMessages.appendChild(renderUserBubble(trimmed));
        stickToBottom = true;
        scrollToBottom(false);

        var apiMessages = chat.history.filter(function (m) {
            return (
                m.role === "user" || m.role === "assistant" || m.role === "tool"
            );
        });

        pendingPosts[chat.id] = true;
        refreshBusy();

        fetch("/api/chat", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                messages: apiMessages,
                currentFlag: currentFlagName(),
                model: selectedModel,
            }),
        })
            .then(function (resp) {
                if (!resp.ok) {
                    return resp.text().then(function (text) {
                        throw new Error(
                            text || "Request failed (" + resp.status + ").",
                        );
                    });
                }
                return resp.json();
            })
            .then(function (data) {
                if (!data || !data.streamId) {
                    throw new Error("Missing stream id from server.");
                }
                attachStream(chat, data.streamId);
            })
            .catch(function (err) {
                var message =
                    err && err.message
                        ? err.message
                        : "Could not reach the assistant.";
                chat.history.push({ role: "error", content: message });
                chat.updatedAt = Date.now();
                saveChats();
                renderChatList();
                if (chat.id === activeChatId) {
                    chatMessages.appendChild(renderErrorBubble(message));
                    maybeScroll();
                }
            })
            .finally(function () {
                delete pendingPosts[chat.id];
                refreshBusy();
                focusChatInput();
            });
    }

    /* ============================================================
       Panel + auth
       ============================================================ */

    function currentFlagName() {
        var match = window.location.pathname.match(/^\/edit\/([^/]+)/);
        return match ? decodeURIComponent(match[1]) : "";
    }

    function openPanel() {
        if (chatOverlay) chatOverlay.classList.add("is-open");
        document.body.style.overflow = "hidden";
        renderChatList();
        scrollToBottom(true);
        stickToBottom = true;
        focusChatInput();
    }

    function closePanel() {
        if (chatOverlay) chatOverlay.classList.remove("is-open");
        document.body.style.overflow = "";
    }

    function focusChatInput() {
        if (!chatInput || !connected || busy || chatInput.disabled) return;
        requestAnimationFrame(function () {
            chatInput.focus();
        });
    }

    function autoResizeChatInput() {
        if (!chatInput) return;
        chatInput.style.height = "auto";
        var next = Math.min(chatInput.scrollHeight, 120);
        chatInput.style.height = next + "px";
    }

    function resetChatInput() {
        if (!chatInput) return;
        chatInput.value = "";
        chatInput.style.height = "auto";
        autoResizeChatInput();
    }

    function isActiveChatStreaming() {
        if (!activeChatId) return false;
        return activeChatId in activeStreams || activeChatId in pendingPosts;
    }

    function refreshBusy() {
        var isBusy = isActiveChatStreaming();
        busy = !!isBusy;
        if (chatInput) chatInput.disabled = !!isBusy || !connected;
        if (chatStatus) {
            chatStatus.textContent = isBusy ? "Streaming…" : "";
            chatStatus.classList.toggle("is-busy", !!isBusy);
        }
        refreshGlobalStreamingIndicator();
    }

    function refreshGlobalStreamingIndicator() {
        var anyActive =
            Object.keys(activeStreams).length > 0 ||
            Object.keys(pendingPosts).length > 0;
        if (chatToggle) chatToggle.classList.toggle("is-streaming", anyActive);
        if (chatShortcutHint)
            chatShortcutHint.classList.toggle("is-streaming", anyActive);
    }

    function setConnected(isConnected) {
        connected = isConnected;
        if (chatConnect) chatConnect.hidden = isConnected;
        if (chatForm) chatForm.hidden = !isConnected;
        if (authLogoutBtn) authLogoutBtn.hidden = !isConnected;
        if (chatInput) chatInput.disabled = busy || !isConnected;
        updateModelTriggerLabel();
        if (!isConnected) {
            closeModelPicker();
            modelsCache = null;
            modelsLoading = null;
            modelsError = null;
        } else {
            // Warm the cache so the trigger label can show the friendly name.
            fetchModels().catch(function () {});
            // Now that we're authenticated, recover any in-flight streams the
            // server is still buffering for this browser.
            resumeActiveStreams();
        }
        refreshBusy();
        if (isConnected) focusChatInput();
    }

    function fetchAuthStatus() {
        return fetch("/api/auth/status")
            .then(function (r) {
                return r.json();
            })
            .then(function (data) {
                setConnected(!!data.connected);
            })
            .catch(function () {
                setConnected(false);
            });
    }

    /* ============================================================
       Shortcuts
       ============================================================ */

    function isMacPlatform() {
        return /Mac|iPhone|iPad|iPod/i.test(navigator.userAgent);
    }

    function shortcutKeyLabel(key) {
        if (key === "mod") return isMacPlatform() ? "⌘" : "Ctrl";
        if (key === "shift") return isMacPlatform() ? "⇧" : "Shift";
        return key;
    }

    function renderShortcutHint(el, keys) {
        if (!el) return;
        el.innerHTML = keys
            .map(function (key) {
                return "<kbd>" + shortcutKeyLabel(key) + "</kbd>";
            })
            .join("");
    }

    function shortcutAccessibleLabel(keys) {
        return keys
            .map(function (key) {
                if (key === "mod")
                    return isMacPlatform() ? "Command" : "Control";
                if (key === "shift") return "Shift";
                return key;
            })
            .join("+");
    }

    function setupShortcutHints() {
        renderShortcutHint(chatShortcutHint, ["mod", "K"]);
        renderShortcutHint(themeShortcutHint, ["mod", "shift", "L"]);
        var chatLabel = shortcutAccessibleLabel(["mod", "K"]);
        var themeLabel = shortcutAccessibleLabel(["mod", "shift", "L"]);
        if (chatShortcutHint) {
            chatShortcutHint.setAttribute(
                "aria-label",
                "Assistant chat, " + chatLabel,
            );
            chatShortcutHint.title = "Assistant chat (" + chatLabel + ")";
        }
        if (themeShortcutHint) {
            themeShortcutHint.setAttribute(
                "aria-label",
                "Toggle theme, " + themeLabel,
            );
            themeShortcutHint.title = "Toggle theme (" + themeLabel + ")";
        }
        if (chatToggle) {
            chatToggle.setAttribute(
                "aria-label",
                "Assistant chat, " + chatLabel,
            );
            chatToggle.title = "Assistant chat (" + chatLabel + ")";
        }
        if (themeToggle) {
            themeToggle.setAttribute(
                "aria-label",
                "Toggle theme, " + themeLabel,
            );
            themeToggle.title = "Toggle theme (" + themeLabel + ")";
        }
    }

    function isTextInputTarget(target) {
        if (!target) return false;
        var tag = target.tagName;
        return (
            tag === "INPUT" ||
            tag === "TEXTAREA" ||
            tag === "SELECT" ||
            target.isContentEditable
        );
    }

    function toggleChatPanel() {
        if (chatOverlay && chatOverlay.classList.contains("is-open")) {
            closePanel();
        } else {
            openPanel();
            fetchAuthStatus().then(focusChatInput);
        }
    }

    function wireKeyboardShortcuts() {
        document.addEventListener("keydown", function (e) {
            var mod = e.metaKey || e.ctrlKey;
            if (
                mod &&
                !e.shiftKey &&
                !e.altKey &&
                e.key.toLowerCase() === "k"
            ) {
                e.preventDefault();
                toggleChatPanel();
                return;
            }
            if (
                mod &&
                e.shiftKey &&
                !e.altKey &&
                e.key.toLowerCase() === "l" &&
                !isTextInputTarget(e.target)
            ) {
                e.preventDefault();
                if (themeToggle) themeToggle.click();
            }
        });
    }

    function wireEvents() {
        if (chatToggle) {
            chatToggle.addEventListener("click", toggleChatPanel);
        }

        if (chatShortcutHint) {
            chatShortcutHint.addEventListener("click", toggleChatPanel);
        }

        if (themeShortcutHint && themeToggle) {
            themeShortcutHint.addEventListener("click", function () {
                themeToggle.click();
            });
        }

        if (chatClose) {
            chatClose.addEventListener("click", closePanel);
        }

        if (chatOverlay) {
            var backdrop = chatOverlay.querySelector(".assistant-backdrop");
            if (backdrop) {
                backdrop.addEventListener("click", closePanel);
            }
        }

        if (chatNewBtn) {
            chatNewBtn.addEventListener("click", function () {
                newChat();
            });
        }

        if (chatForm) {
            chatForm.addEventListener("submit", function (e) {
                e.preventDefault();
                if (!chatInput) return;
                var text = chatInput.value;
                resetChatInput();
                sendMessage(text);
            });
        }

        if (chatInput) {
            chatInput.addEventListener("input", autoResizeChatInput);
            chatInput.addEventListener("keydown", function (e) {
                if (e.key === "Enter" && !e.shiftKey) {
                    e.preventDefault();
                    chatForm &&
                        chatForm.dispatchEvent(
                            new Event("submit", { cancelable: true }),
                        );
                }
            });
            autoResizeChatInput();
        }

        if (authLoginBtn) {
            authLoginBtn.addEventListener("click", function () {
                window.location.href = "/auth/openrouter/start";
            });
        }

        if (authLogoutBtn) {
            authLogoutBtn.addEventListener("click", function () {
                fetch("/api/auth/logout", { method: "POST" })
                    .then(function () {
                        setConnected(false);
                    })
                    .catch(function () {
                        setConnected(false);
                    });
            });
        }

        document.addEventListener("keydown", function (e) {
            if (e.key === "Escape") {
                closePanel();
            }
        });
    }

    function boot() {
        loadChats();
        setupShortcutHints();
        wireScrollTracking();
        wireEvents();
        wireKeyboardShortcuts();
        wireModelPicker();
        updateModelTriggerLabel();
        renderHistory();
        renderChatList();
        refreshBusy();
        fetchAuthStatus();

        var params = new URLSearchParams(window.location.search);
        if (params.get("auth") === "success") {
            setConnected(true);
            openPanel();
            window.history.replaceState({}, "", window.location.pathname);
        }
    }

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", boot);
    } else {
        boot();
    }
})();
