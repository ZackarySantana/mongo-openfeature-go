/* ============================================================
   Flag editor client script
   - Targeted DOM updates (no full re-render on edits)
   - Grouped "Add rule" popover
   - Composite nesting accents + collapse
   - JSON inline errors + Format helper
   - Debounced sync to hidden <textarea>
   - Two-click confirm-delete pattern (with htmx)
   - Toast bridge (HX-Trigger -> #toast-region)
   - Unsaved-changes guard on the edit form
   - Live filter for the flag list
   ============================================================ */

(function () {
    "use strict";

    const RULE_TYPES = [
        {
            label: "Identity & matching",
            options: [
                { type: "ExactMatchRule", desc: "Key equals a specific value" },
                { type: "RegexRule", desc: "Key matches a regular expression" },
                { type: "ExistsRule", desc: "Key is present in the context" },
                { type: "PrefixRule", desc: "Key starts with a prefix" },
                { type: "SuffixRule", desc: "Key ends with a suffix" },
                { type: "ContainsRule", desc: "Key contains a substring" },
                { type: "InListRule", desc: "Key is in a list of values" },
            ],
        },
        {
            label: "Numeric",
            options: [
                { type: "RangeRule", desc: "Numeric key within a range" },
                { type: "FractionalRule", desc: "Random percentage rollout" },
            ],
        },
        {
            label: "Network & geo",
            options: [
                { type: "IPRangeRule", desc: "Key matches a CIDR range" },
                { type: "GeoFenceRule", desc: "Lat/lng within a radius" },
            ],
        },
        {
            label: "Time",
            options: [
                { type: "DateTimeRule", desc: "Within a date and time window" },
                { type: "SemVerRule", desc: "Semantic version constraint" },
                { type: "CronRule", desc: "Active during a cron window" },
            ],
        },
        {
            label: "Composite",
            options: [
                { type: "AndRule", desc: "All child rules must match" },
                { type: "OrRule", desc: "Any child rule must match" },
                { type: "NotRule", desc: "Invert a child rule" },
            ],
        },
        {
            label: "Override",
            options: [
                {
                    type: "OverrideRule",
                    desc: "Force a variant regardless of rules",
                },
            ],
        },
    ];

    const COMPOSITE_TYPES = new Set(["andRule", "orRule", "notRule"]);
    const DOC_BASE =
        "https://github.com/ZackarySantana/mongo-openfeature-go?tab=readme-ov-file";

    // Convert "ExactMatchRule" -> "exactMatchRule" (lower-camel for JSON key).
    function toJsonKey(typeName) {
        if (typeName === "IPRangeRule") return "ipRangeRule";
        if (typeName === "SemVerRule") return "semVerRule";
        return typeName.charAt(0).toLowerCase() + typeName.slice(1);
    }

    function debounce(fn, ms) {
        let t;
        return function () {
            clearTimeout(t);
            const args = arguments;
            t = setTimeout(() => fn.apply(null, args), ms);
        };
    }

    /* ============================================================
       Toast bridge
       ============================================================ */

    function showToast(opts) {
        opts = opts || {};
        const region = document.getElementById("toast-region");
        if (!region) return;
        const el = document.createElement("div");
        el.className =
            "toast toast--" + (opts.kind === "error" ? "error" : "success");
        const body = document.createElement("div");
        if (opts.title) {
            const t = document.createElement("div");
            t.className = "toast__title";
            t.textContent = opts.title;
            body.appendChild(t);
        }
        if (opts.body) {
            const b = document.createElement("div");
            b.className = "toast__body";
            b.textContent = opts.body;
            body.appendChild(b);
        }
        el.appendChild(body);
        region.appendChild(el);
        setTimeout(() => {
            el.classList.add("is-leaving");
            setTimeout(() => el.remove(), 220);
        }, 3200);
    }

    // htmx fires CustomEvent for each name in HX-Trigger.
    document.body &&
        document.body.addEventListener("showToast", function (evt) {
            showToast(evt.detail || {});
        });
    document.addEventListener("DOMContentLoaded", function () {
        document.body.addEventListener("showToast", function (evt) {
            showToast(evt.detail || {});
        });
    });

    /* ============================================================
       Confirm-delete button (works standalone or with htmx)
       ============================================================ */

    function wireConfirmButton(btn) {
        if (btn.__confirmWired) return;
        btn.__confirmWired = true;
        const originalText = btn.textContent.trim();
        let armTimer = null;

        function disarm() {
            btn.dataset.confirm = "idle";
            btn.textContent = originalText;
            if (armTimer) {
                clearTimeout(armTimer);
                armTimer = null;
            }
        }

        btn.addEventListener(
            "click",
            function (evt) {
                if (btn.dataset.confirm !== "armed") {
                    evt.preventDefault();
                    evt.stopImmediatePropagation();
                    btn.dataset.confirm = "armed";
                    btn.textContent = "Click again to delete";
                    armTimer = setTimeout(disarm, 3000);
                    return;
                }
                disarm();
                btn.dispatchEvent(
                    new CustomEvent("confirmed", { bubbles: true }),
                );
                // Visual fade for list rows after htmx removes them.
                const row = btn.closest(".flag-row");
                if (row) row.classList.add("is-removing");
            },
            true,
        );

        btn.addEventListener("blur", function () {
            if (btn.dataset.confirm === "armed") disarm();
        });
    }

    function wireAllConfirmButtons(root) {
        (root || document)
            .querySelectorAll(".confirm-btn")
            .forEach(wireConfirmButton);
    }

    /* ============================================================
       List page: click whole row to open flag
       ============================================================ */

    function wireFlagRow(row) {
        if (row.__flagRowWired) return;
        row.__flagRowWired = true;
        const href = row.dataset.href;
        if (!href) return;

        function navigate() {
            window.location.href = href;
        }

        row.addEventListener("click", function (evt) {
            if (
                evt.target.closest("button, a, input, textarea, select, label")
            ) {
                return;
            }
            navigate();
        });
        row.addEventListener("keydown", function (evt) {
            if (evt.target !== row) return;
            if (evt.key === "Enter" || evt.key === " ") {
                evt.preventDefault();
                navigate();
            }
        });
    }

    function wireAllFlagRows(root) {
        (root || document)
            .querySelectorAll(".flag-row[data-href]")
            .forEach(wireFlagRow);
    }

    /* ============================================================
       List page: live search filter
       ============================================================ */

    function setupListSearch() {
        const box = document.getElementById("search-box");
        const list = document.getElementById("flag-list");
        if (!box || !list) return;

        box.addEventListener("input", function () {
            const q = box.value.trim().toLowerCase();
            list.querySelectorAll("[data-flag-section]").forEach(
                function (section) {
                    let visibleCount = 0;
                    section
                        .querySelectorAll(".flag-row")
                        .forEach(function (row) {
                            const name = (row.dataset.name || "").toLowerCase();
                            const show = !q || name.includes(q);
                            row.style.display = show ? "" : "none";
                            if (show) visibleCount++;
                        });
                    section.hidden = visibleCount === 0;
                },
            );
        });
    }

    /* ============================================================
       List page: new flag dialog
       ============================================================ */

    function sanitizeFlagNameValue(value) {
        let out = "";
        for (let i = 0; i < value.length; i++) {
            const ch = value[i];
            if (out.length === 0) {
                if (/[a-zA-Z]/.test(ch)) out += ch;
            } else if (/[a-zA-Z0-9_-]/.test(ch)) {
                out += ch;
            }
        }
        return out;
    }

    function setupNewFlagDialog() {
        const dialog = document.getElementById("new-flag-dialog");
        const form = document.getElementById("new-flag-form");
        const input = document.getElementById("new-flag-name");
        const errorEl = document.querySelector("[data-new-flag-error]");
        if (!dialog || !form || !input) return;

        const existingNames = new Set();
        try {
            const parsed = JSON.parse(
                dialog.getAttribute("data-flag-names") || "[]",
            );
            if (Array.isArray(parsed)) {
                parsed.forEach(function (name) {
                    existingNames.add(name);
                });
            }
        } catch (e) {
            console.error("Invalid flag name list:", e);
        }

        function showError(message) {
            if (!errorEl) return;
            errorEl.textContent = message;
            errorEl.hidden = false;
            input.setAttribute("aria-invalid", "true");
        }

        function clearError() {
            if (!errorEl) return;
            errorEl.textContent = "";
            errorEl.hidden = true;
            input.removeAttribute("aria-invalid");
        }

        function openDialog() {
            form.reset();
            clearError();
            dialog.showModal();
            input.focus();
        }

        function closeDialog() {
            dialog.close();
        }

        document
            .querySelectorAll("[data-new-flag-open]")
            .forEach(function (btn) {
                btn.addEventListener("click", openDialog);
            });

        document
            .querySelectorAll("[data-new-flag-cancel]")
            .forEach(function (btn) {
                btn.addEventListener("click", closeDialog);
            });

        dialog.addEventListener("click", function (evt) {
            if (evt.target === dialog) closeDialog();
        });

        dialog.addEventListener("close", clearError);

        input.addEventListener("beforeinput", function (evt) {
            if (
                evt.inputType === "insertFromPaste" ||
                evt.inputType === "insertFromDrop" ||
                evt.data == null ||
                evt.data === ""
            ) {
                return;
            }
            const start = input.selectionStart || 0;
            const end = input.selectionEnd || 0;
            const attempt =
                input.value.slice(0, start) + evt.data + input.value.slice(end);
            if (sanitizeFlagNameValue(attempt) !== attempt) {
                evt.preventDefault();
            }
        });

        input.addEventListener("input", function () {
            const sanitized = sanitizeFlagNameValue(input.value);
            if (sanitized !== input.value) {
                input.value = sanitized;
            }
            clearError();
        });

        form.addEventListener("submit", function (evt) {
            evt.preventDefault();
            const name = input.value.trim();
            if (!name) {
                showError("Enter a flag name.");
                input.focus();
                return;
            }
            if (existingNames.has(name)) {
                showError("A flag with this name already exists.");
                input.focus();
                return;
            }
            window.location.href = "/edit/" + encodeURIComponent(name);
        });
    }

    /* ============================================================
       Edit page: category combobox
       ============================================================ */

    function setupCategoryCombobox() {
        const wrap = document.querySelector("[data-category-combobox]");
        if (!wrap) return;

        const input = wrap.querySelector(".category-combobox__input");
        const toggle = wrap.querySelector(".category-combobox__toggle");
        const menu = wrap.querySelector(".category-combobox__menu");
        const options = Array.from(
            wrap.querySelectorAll(".category-combobox__option"),
        );
        const emptyEl = wrap.querySelector(".category-combobox__empty");
        if (!input || !toggle || !menu) return;

        let activeIndex = -1;

        function setExpanded(open) {
            input.setAttribute("aria-expanded", open ? "true" : "false");
        }

        function markSelected() {
            const current = input.value.trim();
            options.forEach(function (opt) {
                const val = opt.getAttribute("data-value") || "";
                opt.classList.toggle("is-selected", val === current);
            });
        }

        function applyFilter() {
            const q = input.value.trim().toLowerCase();
            let visibleCount = 0;

            options.forEach(function (opt) {
                const value = opt.getAttribute("data-value") || "";
                const label = (
                    opt.querySelector(".category-combobox__option-label")
                        ?.textContent || ""
                ).toLowerCase();
                const meta = (
                    opt.querySelector(".category-combobox__option-meta")
                        ?.textContent || ""
                ).toLowerCase();

                let show;
                if (value === "") {
                    show =
                        !q ||
                        label.includes(q) ||
                        meta.includes(q) ||
                        "none".includes(q);
                } else {
                    show =
                        !q ||
                        value.toLowerCase().includes(q) ||
                        label.includes(q);
                }

                opt.hidden = !show;
                opt.classList.remove("is-active");
                if (show) visibleCount++;
            });

            activeIndex = -1;
            if (emptyEl) {
                emptyEl.hidden = visibleCount > 0;
            }
            markSelected();
        }

        function openMenu() {
            wrap.classList.add("is-open");
            menu.hidden = false;
            setExpanded(true);
            applyFilter();
        }

        function closeMenu() {
            wrap.classList.remove("is-open");
            menu.hidden = true;
            setExpanded(false);
            activeIndex = -1;
            options.forEach(function (opt) {
                opt.classList.remove("is-active");
            });
        }

        function chooseOption(opt) {
            input.value = opt.getAttribute("data-value") || "";
            input.dispatchEvent(new Event("input", { bubbles: true }));
            closeMenu();
            input.focus();
        }

        function visibleOptions() {
            return options.filter(function (opt) {
                return !opt.hidden;
            });
        }

        function moveActive(delta) {
            const visible = visibleOptions();
            if (!visible.length) return;
            activeIndex =
                (activeIndex + delta + visible.length) % visible.length;
            options.forEach(function (opt) {
                opt.classList.remove("is-active");
            });
            visible[activeIndex].classList.add("is-active");
            visible[activeIndex].scrollIntoView({ block: "nearest" });
        }

        toggle.addEventListener("click", function () {
            if (wrap.classList.contains("is-open")) closeMenu();
            else openMenu();
        });

        input.addEventListener("focus", function () {
            openMenu();
        });

        input.addEventListener("input", function () {
            if (!wrap.classList.contains("is-open")) openMenu();
            else applyFilter();
        });

        input.addEventListener("keydown", function (evt) {
            if (evt.key === "Escape") {
                closeMenu();
                return;
            }
            if (evt.key === "ArrowDown") {
                evt.preventDefault();
                if (!wrap.classList.contains("is-open")) openMenu();
                moveActive(1);
                return;
            }
            if (evt.key === "ArrowUp") {
                evt.preventDefault();
                if (!wrap.classList.contains("is-open")) openMenu();
                moveActive(-1);
                return;
            }
            if (evt.key === "Enter") {
                evt.preventDefault();
                const visible = visibleOptions();
                if (
                    wrap.classList.contains("is-open") &&
                    activeIndex >= 0 &&
                    visible[activeIndex]
                ) {
                    chooseOption(visible[activeIndex]);
                } else {
                    closeMenu();
                }
            }
        });

        options.forEach(function (opt) {
            opt.addEventListener("mousedown", function (evt) {
                evt.preventDefault();
            });
            opt.addEventListener("click", function () {
                chooseOption(opt);
            });
        });

        document.addEventListener("mousedown", function (evt) {
            if (!wrap.contains(evt.target)) closeMenu();
        });

        markSelected();
    }

    /* ============================================================
       Edit page: default-value JSON validator + Format button
       ============================================================ */

    function setupJsonFieldBlocks() {
        document.querySelectorAll("[data-json-field]").forEach((field) => {
            const ta = field.querySelector("textarea, input");
            const formatBtn = field.querySelector("[data-json-format]");
            if (!ta) return;

            function validate() {
                if (ta.value.trim() === "") {
                    field.classList.remove("is-invalid");
                    return true;
                }
                try {
                    JSON.parse(ta.value);
                    field.classList.remove("is-invalid");
                    return true;
                } catch (e) {
                    field.classList.add("is-invalid");
                    return false;
                }
            }
            ta.addEventListener("input", validate);
            ta.addEventListener("blur", validate);
            validate();

            if (formatBtn) {
                formatBtn.addEventListener("click", function () {
                    try {
                        const parsed = JSON.parse(ta.value);
                        ta.value = JSON.stringify(parsed, null, 2);
                        field.classList.remove("is-invalid");
                        ta.dispatchEvent(new Event("input", { bubbles: true }));
                    } catch (e) {
                        field.classList.add("is-invalid");
                    }
                });
            }
        });
    }

    /* ============================================================
       Edit page: form dirty-tracker + beforeunload guard
       ============================================================ */

    function setupFormDirtyGuard() {
        const form = document.querySelector("form[data-flag-form]");
        if (!form) return;
        let dirty = false;

        function markDirty() {
            dirty = true;
        }

        function isFlagFormSubmission(elt) {
            if (!elt) return false;
            if (elt === form) return true;
            if (form.contains(elt)) return true;
            return elt.getAttribute && elt.getAttribute("form") === form.id;
        }

        function clearDirtyOnSuccessfulSave(evt) {
            const xhr = evt.detail && evt.detail.xhr;
            const elt = evt.detail && evt.detail.elt;
            if (!xhr || !isFlagFormSubmission(elt)) return;
            if (xhr.status >= 200 && xhr.status < 400) {
                dirty = false;
            }
        }

        // Ignore events from regions opted out via data-no-dirty (e.g. the
        // inline flag tester, which lives inside the form for layout reasons
        // but isn't part of the flag being saved).
        function maybeMarkDirty(evt) {
            if (evt.target && evt.target.closest("[data-no-dirty]")) return;
            markDirty();
        }
        form.addEventListener("input", maybeMarkDirty);
        form.addEventListener("change", maybeMarkDirty);

        // Enter in a single-line field would otherwise submit the form via the
        // first toolbar button ("Save & return to list"), save successfully, and
        // still trip beforeunload because navigation starts before dirty clears.
        form.addEventListener("keydown", function (evt) {
            if (evt.key !== "Enter" || evt.defaultPrevented) return;
            if (evt.target.closest("[data-no-dirty]")) return;
            if (evt.target.closest("[data-category-combobox]")) return;
            if (evt.target.tagName === "TEXTAREA") return;
            if (evt.target.type === "submit") return;
            evt.preventDefault();
        });

        window.addEventListener("beforeunload", function (e) {
            if (!dirty) return;
            e.preventDefault();
            e.returnValue = "";
            return "";
        });

        // Clear dirty before htmx follows HX-Redirect so beforeunload does not fire
        // after a successful save.
        document.body.addEventListener(
            "htmx:beforeOnLoad",
            clearDirtyOnSuccessfulSave,
        );
        document.body.addEventListener(
            "htmx:afterRequest",
            clearDirtyOnSuccessfulSave,
        );

        // Internal hook: rule-builder can flag dirty too.
        form.__markDirty = markDirty;
    }

    /* ============================================================
       Inline flag tester (right column on the edit page)
       ============================================================ */

    function setupTester() {
        const card = document.querySelector(".tester-card");
        if (!card) return;
        const rowsHost = card.querySelector("[data-tester-rows]");
        const emptyEl = card.querySelector("[data-tester-empty]");
        const subtitleEl = card.querySelector("[data-tester-subtitle]");
        const hintEl = card.querySelector("[data-tester-hint]");
        if (!rowsHost) return;

        const copy = {
            saved: {
                subtitle: "Uses the version stored on the server",
                empty: "No context keys in the saved rules for this flag. Run test to evaluate with an empty context.",
            },
            draft: {
                subtitle: "Uses your unsaved changes from the editor",
                empty: "No context keys in your current rule edits. Run test to evaluate with an empty context.",
            },
        };

        function collectContextValues() {
            const values = {};
            card.querySelectorAll("[data-tester-row]").forEach(function (row) {
                const key = row.getAttribute("data-context-key");
                const valEl = row.querySelector("[data-tester-value]");
                if (key && valEl) values[key] = valEl.value;
            });
            return values;
        }

        function renderContextRows(fields) {
            const values = collectContextValues();
            rowsHost.innerHTML = "";

            if (!fields.length) {
                rowsHost.hidden = true;
                if (emptyEl) {
                    emptyEl.hidden = false;
                    emptyEl.textContent = copy[getTesterSource()].empty;
                }
                return;
            }

            rowsHost.hidden = false;
            if (emptyEl) emptyEl.hidden = true;

            fields.forEach(function (field) {
                const row = document.createElement("div");
                row.className = "tester-field";
                row.setAttribute("data-tester-row", "");
                row.setAttribute("data-context-key", field.key);

                const label = document.createElement("label");
                label.className = "tester-row__key";
                label.textContent = field.key;

                const input = document.createElement("input");
                input.className = "input input--mono tester-row__value";
                input.setAttribute("data-tester-value", "");
                input.type = "text";
                input.placeholder = "Leave empty to omit";
                input.autocomplete = "off";
                input.setAttribute("aria-label", "Value for " + field.key);
                if (values[field.key] != null) {
                    input.value = values[field.key];
                }

                row.appendChild(label);
                row.appendChild(input);

                if (field.rules && field.rules.length) {
                    const refs = document.createElement("ul");
                    refs.className = "tester-field__refs";
                    refs.setAttribute(
                        "aria-label",
                        "Rules that use " + field.key,
                    );
                    field.rules.forEach(function (ref) {
                        const li = document.createElement("li");
                        const link = document.createElement("a");
                        link.href = "#";
                        link.className = "tester-field__ref";
                        link.setAttribute("data-rule-link", "");
                        link.setAttribute(
                            "data-rule-index",
                            String(ref.topLevelIndex),
                        );
                        link.title = "Scroll to this rule";
                        link.textContent = ref.label;
                        li.appendChild(link);
                        refs.appendChild(li);
                    });
                    row.appendChild(refs);
                }

                rowsHost.appendChild(row);
            });

            wireTestResultLinks(rowsHost);
        }

        function refreshTesterContextUI() {
            const source = getTesterSource();
            const text = copy[source] || copy.saved;
            if (subtitleEl) subtitleEl.textContent = text.subtitle;
            if (hintEl) {
                hintEl.innerHTML =
                    source === "saved"
                        ? "Context keys come from the saved rules. Only fields you fill in are sent. Leave a value empty to simulate that key missing from the context. Values are parsed as JSON when possible (<code>42</code>, <code>true</code>, <code>[1,2]</code>). RFC3339 strings become timestamps."
                        : "Context keys come from your current rule edits. Only fields you fill in are sent. Leave a value empty to simulate that key missing from the context. Values are parsed as JSON when possible (<code>42</code>, <code>true</code>, <code>[1,2]</code>). RFC3339 strings become timestamps.";
            }

            const fields =
                source === "draft"
                    ? parseDraftContextKeyFields()
                    : parseSavedContextKeyFields(card);
            renderContextRows(fields);
        }

        card.querySelectorAll("[data-tester-source]").forEach(function (input) {
            input.addEventListener("change", refreshTesterContextUI);
        });

        const builder = document.getElementById("rule-builder");
        const refreshDraftTesterContext = debounce(function () {
            if (getTesterSource() === "draft") {
                refreshTesterContextUI();
            }
        }, 200);
        if (builder) {
            builder.addEventListener("input", refreshDraftTesterContext);
            builder.addEventListener("rules-changed", function () {
                if (getTesterSource() === "draft") {
                    refreshTesterContextUI();
                }
            });
        }

        refreshTesterContextUI();

        document.body.addEventListener("htmx:configRequest", function (evt) {
            const elt = evt.detail && evt.detail.elt;
            if (!elt) return;
            const form = document.getElementById("flag-form");
            const syncRules =
                typeof window.syncEditorRulesForTest === "function"
                    ? window.syncEditorRulesForTest
                    : null;
            if (elt.classList && elt.classList.contains("tester-run")) {
                if (syncRules) syncRules();
                return;
            }
            if (!form || !syncRules) return;
            const isSave =
                elt === form ||
                form.contains(elt) ||
                (elt.getAttribute && elt.getAttribute("form") === form.id);
            if (isSave) syncRules();
        });

        document.body.addEventListener("htmx:afterRequest", function (evt) {
            const xhr = evt.detail && evt.detail.xhr;
            const elt = evt.detail && evt.detail.elt;
            const form = document.getElementById("flag-form");
            if (!xhr || !form || xhr.status < 200 || xhr.status >= 400) return;
            const isSave =
                elt === form ||
                form.contains(elt) ||
                (elt.getAttribute && elt.getAttribute("form") === form.id);
            if (!isSave) return;
            syncSavedContextKeyFields(card);
            refreshTesterContextUI();
        });

        // Enter inside a value field is a power-user shortcut for "Run test".
        rowsHost.addEventListener("keydown", function (evt) {
            if (evt.key !== "Enter") return;
            if (!evt.target.closest("[data-tester-value]")) return;
            evt.preventDefault();
            const runBtn = card.querySelector(".tester-run");
            if (runBtn) runBtn.click();
        });
    }

    // Scroll a rule into view, expand collapsed ancestors, and flash it briefly.
    function revealRule(el) {
        if (!el) return;
        var parent = el.parentElement;
        while (parent && parent.id !== "rule-builder") {
            if (
                parent.classList &&
                parent.classList.contains("rule") &&
                parent.classList.contains("is-collapsed")
            ) {
                parent.classList.remove("is-collapsed");
            }
            parent = parent.parentElement;
        }
        el.classList.remove("is-collapsed");
        el.scrollIntoView({ behavior: "smooth", block: "start" });
        el.classList.remove("rule--toc-flash");
        void el.offsetWidth;
        el.classList.add("rule--toc-flash");
        window.setTimeout(function () {
            el.classList.remove("rule--toc-flash");
        }, 1200);
    }

    // Scroll to a top-level rule in the editor, mirroring the rules overview links.
    function scrollToRule(index) {
        revealRule(
            document.querySelector(
                '.rules-card .rule[data-toplevel-index="' + index + '"]',
            ),
        );
    }

    function wireTestResultLinks(root) {
        (root || document)
            .querySelectorAll("[data-rule-link]")
            .forEach(function (link) {
                if (link.__ruleLinkWired) return;
                link.__ruleLinkWired = true;
                link.addEventListener("click", function (evt) {
                    evt.preventDefault();
                    const idx = link.getAttribute("data-rule-index");
                    if (idx !== null && idx !== "") scrollToRule(idx);
                });
            });
    }

    // Serialize the tester rows into a JSON string. Keys are fixed per rule;
    // only rows with a non-empty value are included in the context object.
    // Values are parsed as JSON when possible (so `42`, `true`, `[1,2]` all
    // become their typed values) and otherwise treated as plain strings.
    // RFC3339 strings get upgraded to time.Time by the server so date/cron
    // rules work. Exposed on `window` so it's reachable from htmx's
    // `hx-vals='js:...'` attribute.
    window.buildTesterContext = function buildTesterContext() {
        const rows = document.querySelectorAll(
            ".tester-card [data-tester-row]",
        );
        const obj = {};
        rows.forEach(function (row) {
            const key = row.getAttribute("data-context-key");
            const valEl = row.querySelector("[data-tester-value]");
            if (!key || !valEl) return;
            const raw = valEl.value.trim();
            if (raw === "") return;
            try {
                obj[key] = JSON.parse(raw);
            } catch (e) {
                obj[key] = raw;
            }
        });
        return JSON.stringify(obj);
    };

    function getTesterSource() {
        const checked = document.querySelector(
            ".tester-card [data-tester-source]:checked",
        );
        return checked ? checked.value : "draft";
    }
    window.getTesterSource = getTesterSource;

    function ruleDataTypeKey(ruleData) {
        return Object.keys(ruleData).find(function (k) {
            return k.charAt(0) !== "_";
        });
    }

    function directContextKeys(ruleTypeKey, rule) {
        if (!rule) return [];
        switch (ruleTypeKey) {
            case "exactMatchRule":
            case "regexRule":
            case "existsRule":
            case "fractionalRule":
            case "rangeRule":
            case "inListRule":
            case "prefixRule":
            case "suffixRule":
            case "containsRule":
            case "ipRangeRule":
            case "dateTimeRule":
            case "semVerRule":
                return rule.Key ? [rule.Key] : [];
            case "geoFenceRule": {
                const keys = [];
                if (rule.LatKey) keys.push(rule.LatKey);
                if (rule.LngKey) keys.push(rule.LngKey);
                return keys;
            }
            case "cronRule":
                return rule.Key ? [rule.Key] : [];
            default:
                return [];
        }
    }

    function formatContextKeyRefLabel(topLevelIndex, ruleTypeKey, nestedIn) {
        if (nestedIn) {
            return (
                "#" + (topLevelIndex + 1) + " " + nestedIn + " · " + ruleTypeKey
            );
        }
        return "#" + (topLevelIndex + 1) + " " + ruleTypeKey;
    }

    function appendContextKeyRef(byKey, key, ref) {
        if (!byKey[key]) byKey[key] = [];
        const exists = byKey[key].some(function (existing) {
            return (
                existing.topLevelIndex === ref.topLevelIndex &&
                existing.label === ref.label
            );
        });
        if (!exists) byKey[key].push(ref);
    }

    function walkRuleForContextKeys(ruleData, topLevelIndex, nestedIn, byKey) {
        const ruleTypeKey = ruleDataTypeKey(ruleData);
        if (!ruleTypeKey) return;
        const rule = ruleData[ruleTypeKey];

        directContextKeys(ruleTypeKey, rule).forEach(function (key) {
            appendContextKeyRef(byKey, key, {
                topLevelIndex: topLevelIndex,
                label: formatContextKeyRefLabel(
                    topLevelIndex,
                    ruleTypeKey,
                    nestedIn,
                ),
            });
        });

        if (
            (ruleTypeKey === "andRule" || ruleTypeKey === "orRule") &&
            Array.isArray(rule.Rules)
        ) {
            rule.Rules.forEach(function (child) {
                walkRuleForContextKeys(
                    child,
                    topLevelIndex,
                    ruleTypeKey,
                    byKey,
                );
            });
        } else if (ruleTypeKey === "notRule" && rule.Rule) {
            walkRuleForContextKeys(
                rule.Rule,
                topLevelIndex,
                ruleTypeKey,
                byKey,
            );
        }
    }

    function collectContextKeyFieldsFromRules(rules) {
        const byKey = {};
        if (!Array.isArray(rules)) return [];
        rules.forEach(function (ruleData, index) {
            walkRuleForContextKeys(ruleData, index, "", byKey);
        });
        return Object.keys(byKey)
            .sort()
            .map(function (key) {
                return { key: key, rules: byKey[key] };
            });
    }

    function parseSavedContextKeyFields(card) {
        const raw = card.getAttribute("data-saved-context-fields");
        if (!raw) return [];
        try {
            const parsed = JSON.parse(raw);
            if (!Array.isArray(parsed)) return [];
            return parsed.map(function (field) {
                return {
                    key: field.Key,
                    rules: (field.Rules || []).map(function (ref) {
                        return {
                            topLevelIndex: ref.TopLevelIndex,
                            label: ref.Label,
                        };
                    }),
                };
            });
        } catch (e) {
            return [];
        }
    }

    function parseDraftContextKeyFields() {
        const rulesEl = document.getElementById("rules");
        if (!rulesEl) return [];
        try {
            const parsed = JSON.parse(rulesEl.value || "[]");
            return collectContextKeyFieldsFromRules(parsed);
        } catch (e) {
            return [];
        }
    }

    function contextKeyFieldsToJSON(fields) {
        return JSON.stringify(
            fields.map(function (field) {
                return {
                    Key: field.key,
                    Rules: (field.rules || []).map(function (ref) {
                        return {
                            TopLevelIndex: ref.topLevelIndex,
                            Label: ref.label,
                        };
                    }),
                };
            }),
        );
    }

    function syncSavedContextKeyFields(card) {
        if (typeof window.syncEditorRulesForTest === "function") {
            window.syncEditorRulesForTest();
        }
        const fields = parseDraftContextKeyFields();
        card.setAttribute(
            "data-saved-context-fields",
            contextKeyFieldsToJSON(fields),
        );
        return fields;
    }

    /* ============================================================
       Rule builder
       ============================================================ */

    function setupRuleBuilder() {
        const textarea = document.getElementById("rules");
        const container = document.getElementById("rule-builder");
        if (!textarea || !container) return;

        textarea.style.display = "none";

        let state = [];
        try {
            const parsed = JSON.parse(textarea.value);
            if (Array.isArray(parsed)) state = parsed;
        } catch (e) {
            console.error("Invalid initial rules JSON:", e);
        }

        const form = textarea.closest("form");
        const markDirty =
            form && form.__markDirty ? form.__markDirty : function () {};

        const sync = debounce(function () {
            textarea.value = JSON.stringify(state, null, 2);
        }, 120);

        function syncNow() {
            textarea.value = JSON.stringify(state, null, 2);
        }

        // Touch immediately so the hidden textarea always has clean JSON.
        syncNow();
        window.syncEditorRulesForTest = syncNow;

        let nextRuleId = 1;

        function ensureRuleId(ruleData) {
            if (!ruleData.__tocId) {
                ruleData.__tocId = "rule-" + nextRuleId++;
            }
            return ruleData.__tocId;
        }

        function updateRuleTocMeta(el, ruleData) {
            const ruleTypeKey = Object.keys(ruleData)[0];
            const rule = ruleData[ruleTypeKey];
            let meta = "";
            if (COMPOSITE_TYPES.has(ruleTypeKey)) {
                meta = computeVariant(ruleData);
            } else if (rule.VariantID) {
                meta = String(rule.VariantID);
            }
            el.dataset.tocType = ruleTypeKey;
            el.dataset.tocMeta = meta;
        }

        function getRuleDepth(ruleEl) {
            let depth = 0;
            let el = ruleEl.parentElement;
            while (el && el.id !== "rule-builder") {
                if (el.classList.contains("rule-list--nested")) depth++;
                el = el.parentElement;
            }
            return depth;
        }

        function refreshRuleTOC() {
            const tocNav = document.getElementById("rule-toc");
            const builder = document.getElementById("rule-builder");
            if (!tocNav || !builder) return;

            const rules = builder.querySelectorAll(".rule");
            tocNav.innerHTML = "";

            if (!rules.length) {
                const p = document.createElement("p");
                p.className = "rule-toc__empty";
                p.id = "rule-toc-empty";
                p.textContent = "No rules yet";
                tocNav.appendChild(p);
                return;
            }

            const list = document.createElement("ul");
            list.className = "rule-toc__list";

            let index = 0;
            rules.forEach(function (el) {
                if (el.__ruleData) updateRuleTocMeta(el, el.__ruleData);
                index++;

                const depth = getRuleDepth(el);
                const type = el.dataset.tocType || el.dataset.type || "rule";
                const meta = el.dataset.tocMeta || "";

                const li = document.createElement("li");
                li.className = "rule-toc__item";
                li.style.setProperty("--toc-depth", String(depth));

                const link = document.createElement("a");
                link.href = "#" + el.id;
                link.className = "rule-toc__link";
                link.setAttribute("data-depth", String(depth));

                const idxSpan = document.createElement("span");
                idxSpan.className = "rule-toc__index";
                idxSpan.textContent = String(index);

                const typeSpan = document.createElement("span");
                typeSpan.className = "rule-toc__type";
                typeSpan.textContent = type;

                link.appendChild(idxSpan);
                link.appendChild(typeSpan);
                if (meta) {
                    const metaSpan = document.createElement("span");
                    metaSpan.className = "rule-toc__meta";
                    metaSpan.textContent = meta;
                    link.appendChild(metaSpan);
                }

                link.addEventListener("click", function (evt) {
                    evt.preventDefault();
                    revealRule(el);
                });

                li.appendChild(link);
                list.appendChild(li);
            });

            tocNav.appendChild(list);
            applyRuleSearch();
        }

        function ruleSearchText(el) {
            const type = (
                el.dataset.tocType ||
                el.dataset.type ||
                ""
            ).toLowerCase();
            const meta = (el.dataset.tocMeta || "").toLowerCase();
            const body = el.querySelector(":scope > .rule__body");
            if (!body) return (type + " " + meta).trim();

            // Only fields on this rule, not nested rules or the "Add rule" picker.
            // picker menu (which lists every type name, e.g. "RegexRule").
            const clone = body.cloneNode(true);
            clone.querySelectorAll(".rule, .add-rule").forEach(function (n) {
                n.remove();
            });
            return (type + " " + meta + " " + clone.textContent).toLowerCase();
        }

        function applyRuleSearch() {
            const builder = document.getElementById("rule-builder");
            const box = document.getElementById("rules-search-box");
            const emptyMsg = document.getElementById("rules-search-empty");
            if (!builder) return;

            const q = box ? box.value.trim().toLowerCase() : "";
            const rules = Array.from(builder.querySelectorAll(".rule"));
            const matched = new Set();

            if (!q) {
                rules.forEach(function (el) {
                    el.classList.remove("rule--search-hidden");
                });
                document
                    .querySelectorAll(".rule-toc__item")
                    .forEach(function (item) {
                        item.classList.remove("rule-toc__item--search-hidden");
                    });
                if (emptyMsg) emptyMsg.hidden = true;
                return;
            }

            rules.forEach(function (el) {
                if (ruleSearchText(el).includes(q)) matched.add(el);
            });

            rules.forEach(function (el) {
                if (!matched.has(el)) return;
                let parent = el.parentElement;
                while (parent && parent !== builder) {
                    if (parent.classList.contains("rule")) {
                        matched.add(parent);
                    }
                    parent = parent.parentElement;
                }
            });

            let visibleCount = 0;
            rules.forEach(function (el) {
                const show = matched.has(el);
                el.classList.toggle("rule--search-hidden", !show);
                if (show) visibleCount++;
            });

            document
                .querySelectorAll(".rule-toc__item")
                .forEach(function (item) {
                    const link = item.querySelector(".rule-toc__link");
                    const href = link && link.getAttribute("href");
                    const id = href ? href.slice(1) : "";
                    const ruleEl = id ? document.getElementById(id) : null;
                    const show = ruleEl && matched.has(ruleEl);
                    item.classList.toggle(
                        "rule-toc__item--search-hidden",
                        !show,
                    );
                });

            if (emptyMsg) {
                emptyMsg.hidden = visibleCount > 0;
            }
        }

        const scheduleTocRefresh = debounce(refreshRuleTOC, 80);

        function notifyChange() {
            sync();
            markDirty();
            scheduleTocRefresh();
            container.dispatchEvent(
                new CustomEvent("rules-changed", { bubbles: true }),
            );
        }

        // ----- Field factories -----

        function field(label, child, opts) {
            opts = opts || {};
            const wrap = document.createElement("div");
            wrap.className = "field";
            if (label) {
                const l = document.createElement("label");
                l.className = "field__label";
                l.textContent = label;
                wrap.appendChild(l);
            }
            wrap.appendChild(child);
            if (opts.error) {
                const e = document.createElement("span");
                e.className = "field__error";
                e.textContent = opts.error;
                wrap.appendChild(e);
            }
            return wrap;
        }

        function textField(label, obj, key, opts) {
            opts = opts || {};
            const input = document.createElement("input");
            input.className = "input";
            input.type = "text";
            input.value = obj[key] != null ? obj[key] : "";
            if (opts.placeholder) input.placeholder = opts.placeholder;
            input.addEventListener("input", function () {
                obj[key] = input.value;
                notifyChange();
                if (key === "VariantID" && opts.onVariantChange)
                    opts.onVariantChange();
            });
            const wrap = field(label, input);
            if (opts.hint) {
                const hint = document.createElement("span");
                hint.className = "field__hint";
                hint.textContent = opts.hint;
                wrap.appendChild(hint);
            }
            return wrap;
        }

        function numberField(label, obj, key, opts) {
            opts = opts || {};
            const input = document.createElement("input");
            input.className = "input";
            input.type = "number";
            input.step = opts.step != null ? opts.step : "any";
            if (opts.min != null) input.min = String(opts.min);
            if (opts.max != null) input.max = String(opts.max);
            input.value = obj[key] != null ? obj[key] : 0;
            input.addEventListener("input", function () {
                const v = parseFloat(input.value);
                obj[key] = isNaN(v) ? 0 : v;
                notifyChange();
            });
            const wrap = field(label, input);
            if (opts.hint) {
                const hint = document.createElement("span");
                hint.className = "field__hint";
                hint.textContent = opts.hint;
                wrap.appendChild(hint);
            }
            return wrap;
        }

        function checkboxField(label, obj, key) {
            const wrap = document.createElement("div");
            wrap.className = "field";
            const lbl = document.createElement("label");
            lbl.className = "checkbox";
            const input = document.createElement("input");
            input.type = "checkbox";
            input.checked = !!obj[key];
            input.addEventListener("change", function () {
                obj[key] = input.checked;
                notifyChange();
            });
            const span = document.createElement("span");
            span.textContent = label;
            lbl.appendChild(input);
            lbl.appendChild(span);
            wrap.appendChild(lbl);
            return wrap;
        }

        function rfc3339ToDateTimeParts(value) {
            if (value == null || value === "") {
                return { date: "", time: "" };
            }
            const d = new Date(String(value));
            if (isNaN(d.getTime())) {
                return { date: "", time: "" };
            }
            const iso = d.toISOString();
            const time = iso.slice(11, 19);
            return {
                date: iso.slice(0, 10),
                time: time === "00:00:00" ? "" : time.slice(0, 5),
            };
        }

        function dateTimePartsToRFC3339(date, time) {
            if (!date) return "";
            let t = (time || "").trim();
            if (t.length === 5) t += ":00";
            if (!t) t = "00:00:00";
            const d = new Date(date + "T" + t + "Z");
            if (isNaN(d.getTime())) return "";
            return d.toISOString().replace(/\.\d{3}Z$/, "Z");
        }

        function dateTimeField(label, obj, key) {
            const wrap = document.createElement("div");
            wrap.className = "field datetime-field";

            const lbl = document.createElement("label");
            lbl.className = "field__label";
            lbl.textContent = label;

            const pickers = document.createElement("div");
            pickers.className = "datetime-field__pickers";

            const dateInput = document.createElement("input");
            dateInput.className = "input datetime-field__date";
            dateInput.type = "date";

            const timeInput = document.createElement("input");
            timeInput.className = "input datetime-field__time";
            timeInput.type = "time";
            timeInput.step = "60";

            const hint = document.createElement("span");
            hint.className = "field__hint";
            hint.textContent =
                "UTC. Time is optional and defaults to midnight.";

            function syncFromObj() {
                const parts = rfc3339ToDateTimeParts(obj[key]);
                dateInput.value = parts.date;
                timeInput.value = parts.time;
            }

            function syncToObj() {
                const rfc = dateTimePartsToRFC3339(
                    dateInput.value,
                    timeInput.value,
                );
                if (!rfc) {
                    delete obj[key];
                } else {
                    obj[key] = rfc;
                }
                notifyChange();
            }

            dateInput.addEventListener("change", syncToObj);
            timeInput.addEventListener("change", syncToObj);

            pickers.appendChild(dateInput);
            pickers.appendChild(timeInput);
            wrap.appendChild(lbl);
            wrap.appendChild(pickers);
            wrap.appendChild(hint);
            syncFromObj();
            return wrap;
        }

        function jsonField(label, obj, key) {
            const wrap = document.createElement("div");
            wrap.className = "field";

            const labelRow = document.createElement("div");
            labelRow.className = "field__label-row";
            const lbl = document.createElement("label");
            lbl.className = "field__label";
            lbl.textContent = label;
            labelRow.appendChild(lbl);

            const formatBtn = document.createElement("button");
            formatBtn.type = "button";
            formatBtn.className = "btn btn--ghost btn--sm";
            formatBtn.textContent = "Format";
            labelRow.appendChild(formatBtn);

            wrap.appendChild(labelRow);

            const ta = document.createElement("textarea");
            ta.className = "textarea textarea--mono";
            ta.spellcheck = false;
            ta.rows = 3;
            let initial = "";
            try {
                initial =
                    obj[key] === undefined
                        ? ""
                        : JSON.stringify(obj[key], null, 2);
            } catch (e) {
                initial = "";
            }
            ta.value = initial;

            const err = document.createElement("span");
            err.className = "field__error";
            err.textContent = "Invalid JSON.";

            function validate() {
                if (ta.value.trim() === "") {
                    delete obj[key];
                    wrap.classList.remove("is-invalid");
                    notifyChange();
                    return;
                }
                try {
                    obj[key] = JSON.parse(ta.value);
                    wrap.classList.remove("is-invalid");
                    notifyChange();
                } catch (e) {
                    wrap.classList.add("is-invalid");
                }
            }

            ta.addEventListener("input", validate);
            formatBtn.addEventListener("click", function () {
                try {
                    const parsed = JSON.parse(ta.value);
                    ta.value = JSON.stringify(parsed, null, 2);
                    wrap.classList.remove("is-invalid");
                } catch (e) {
                    wrap.classList.add("is-invalid");
                }
            });

            wrap.appendChild(ta);
            wrap.appendChild(err);
            return wrap;
        }

        function parseListItemValue(raw) {
            const t = raw.trim();
            if (t === "") return null;
            if (t === "true") return true;
            if (t === "false") return false;
            if (t === "null") return null;
            try {
                return JSON.parse(t);
            } catch (e) {
                /* fall through */
            }
            const n = Number(t);
            if (!isNaN(n) && String(n) === t) return n;
            return t;
        }

        function valueToListItemString(v) {
            if (v === null || v === undefined) return "";
            if (typeof v === "string") return v;
            return JSON.stringify(v);
        }

        function listField(label, obj, key, opts) {
            opts = opts || {};
            const parseItem = !!opts.parseItem;
            const wrap = document.createElement("div");
            wrap.className = "field list-field";

            const lbl = document.createElement("label");
            lbl.className = "field__label";
            lbl.textContent = label;
            wrap.appendChild(lbl);

            const list = document.createElement("div");
            list.className = "list-field__items";
            wrap.appendChild(list);

            if (!Array.isArray(obj[key])) obj[key] = [];

            function commitRow(index, raw) {
                const trimmed = raw.trim();
                if (trimmed === "") {
                    obj[key].splice(index, 1);
                    render();
                } else {
                    obj[key][index] = parseItem
                        ? parseListItemValue(raw)
                        : trimmed;
                }
                notifyChange();
            }

            function render() {
                list.innerHTML = "";
                obj[key].forEach(function (item, index) {
                    const row = document.createElement("div");
                    row.className = "list-field__row";

                    const input = document.createElement("input");
                    input.className = "input input--mono list-field__input";
                    input.type = "text";
                    input.placeholder = opts.placeholder || "Value";
                    input.value = valueToListItemString(item);
                    input.addEventListener("change", function () {
                        commitRow(index, input.value);
                    });
                    input.addEventListener("keydown", function (evt) {
                        if (evt.key === "Enter") {
                            evt.preventDefault();
                            commitRow(index, input.value);
                        }
                    });

                    const removeBtn = document.createElement("button");
                    removeBtn.type = "button";
                    removeBtn.className =
                        "btn btn--ghost btn--sm btn--icon list-field__remove";
                    removeBtn.setAttribute("aria-label", "Remove item");
                    removeBtn.innerHTML =
                        '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>';
                    removeBtn.addEventListener("click", function () {
                        obj[key].splice(index, 1);
                        render();
                        notifyChange();
                    });

                    row.appendChild(input);
                    row.appendChild(removeBtn);
                    list.appendChild(row);
                });

                const addBtn = document.createElement("button");
                addBtn.type = "button";
                addBtn.className = "btn btn--ghost btn--sm list-field__add";
                addBtn.textContent = opts.addLabel || "Add item";
                addBtn.addEventListener("click", function () {
                    obj[key].push(parseItem ? "" : "");
                    render();
                    notifyChange();
                    const lastInput = list.querySelector(
                        ".list-field__row:last-child .list-field__input",
                    );
                    if (lastInput) lastInput.focus();
                });
                list.appendChild(addBtn);
            }

            render();
            if (opts.hint) {
                const hint = document.createElement("span");
                hint.className = "field__hint";
                hint.textContent = opts.hint;
                wrap.appendChild(hint);
            }
            return wrap;
        }

        function percentageField(label, obj, key) {
            const wrap = document.createElement("div");
            wrap.className = "field percentage-field";

            const lbl = document.createElement("label");
            lbl.className = "field__label";
            lbl.textContent = label;

            const control = document.createElement("div");
            control.className = "percentage-field__control";

            const slider = document.createElement("input");
            slider.className = "percentage-field__slider";
            slider.type = "range";
            slider.min = "0";
            slider.max = "100";
            slider.step = "1";

            const number = document.createElement("input");
            number.className = "input percentage-field__number";
            number.type = "number";
            number.min = "0";
            number.max = "100";
            number.step = "1";

            const pct = obj[key] != null ? Number(obj[key]) : 0;
            slider.value = String(pct);
            number.value = String(pct);

            function sync(v) {
                let n = Math.max(0, Math.min(100, Number(v) || 0));
                slider.value = String(n);
                number.value = String(n);
                obj[key] = n;
                notifyChange();
            }

            slider.addEventListener("input", function () {
                sync(slider.value);
            });
            number.addEventListener("input", function () {
                sync(number.value);
            });

            control.appendChild(slider);
            control.appendChild(number);
            wrap.appendChild(lbl);
            wrap.appendChild(control);
            const hint = document.createElement("span");
            hint.className = "field__hint";
            hint.textContent =
                "Percentage of context values that should match.";
            wrap.appendChild(hint);
            return wrap;
        }

        function nsToHoursMinutes(ns) {
            const totalMinutes = Math.floor(Number(ns || 0) / 1e9 / 60);
            return {
                hours: Math.floor(totalMinutes / 60),
                minutes: totalMinutes % 60,
            };
        }

        function hoursMinutesToNs(hours, minutes) {
            return Math.round((hours * 3600 + minutes * 60) * 1e9);
        }

        function durationField(label, obj, key) {
            const wrap = document.createElement("div");
            wrap.className = "field duration-field";

            const lbl = document.createElement("label");
            lbl.className = "field__label";
            lbl.textContent = label;

            const parts = nsToHoursMinutes(obj[key]);
            const pickers = document.createElement("div");
            pickers.className = "duration-field__pickers";

            function makePart(partLabel, value) {
                const part = document.createElement("div");
                part.className = "duration-field__part";
                const partLbl = document.createElement("span");
                partLbl.className = "duration-field__part-label";
                partLbl.textContent = partLabel;
                const input = document.createElement("input");
                input.className = "input";
                input.type = "number";
                input.min = "0";
                input.step = "1";
                input.value = String(value);
                part.appendChild(partLbl);
                part.appendChild(input);
                return { part: part, input: input };
            }

            const hoursPart = makePart("Hours", parts.hours);
            const minutesPart = makePart("Minutes", parts.minutes);
            pickers.appendChild(hoursPart.part);
            pickers.appendChild(minutesPart.part);

            function syncToObj() {
                obj[key] = hoursMinutesToNs(
                    parseInt(hoursPart.input.value, 10) || 0,
                    parseInt(minutesPart.input.value, 10) || 0,
                );
                notifyChange();
            }

            hoursPart.input.addEventListener("input", syncToObj);
            minutesPart.input.addEventListener("input", syncToObj);

            wrap.appendChild(lbl);
            wrap.appendChild(pickers);
            const hint = document.createElement("span");
            hint.className = "field__hint";
            hint.textContent =
                "How long the cron window stays active after each trigger.";
            wrap.appendChild(hint);
            return wrap;
        }

        const CRON_PRESETS = [
            { label: "Every hour", value: "0 * * * *" },
            { label: "Weekdays 9:00", value: "0 9 * * MON-FRI" },
            { label: "Daily midnight", value: "0 0 * * *" },
            { label: "Every 15 min", value: "*/15 * * * *" },
        ];

        function cronSpecField(obj, key) {
            const wrap = document.createElement("div");
            wrap.className = "field cron-field";

            const lbl = document.createElement("label");
            lbl.className = "field__label";
            lbl.textContent = "Schedule";

            const presets = document.createElement("div");
            presets.className = "cron-field__presets";

            const input = document.createElement("input");
            input.className = "input input--mono";
            input.type = "text";
            input.placeholder = "0 9 * * MON-FRI";
            input.value = obj[key] != null ? obj[key] : "";

            function setSpec(value) {
                input.value = value;
                obj[key] = value;
                presets
                    .querySelectorAll(".cron-field__preset")
                    .forEach(function (btn) {
                        btn.classList.toggle(
                            "is-active",
                            btn.getAttribute("data-value") === value,
                        );
                    });
                notifyChange();
            }

            CRON_PRESETS.forEach(function (preset) {
                const btn = document.createElement("button");
                btn.type = "button";
                btn.className = "cron-field__preset";
                btn.setAttribute("data-value", preset.value);
                btn.textContent = preset.label;
                if (obj[key] === preset.value) btn.classList.add("is-active");
                btn.addEventListener("click", function () {
                    setSpec(preset.value);
                });
                presets.appendChild(btn);
            });

            input.addEventListener("input", function () {
                obj[key] = input.value;
                presets
                    .querySelectorAll(".cron-field__preset")
                    .forEach(function (btn) {
                        btn.classList.remove("is-active");
                    });
                notifyChange();
            });

            wrap.appendChild(lbl);
            wrap.appendChild(presets);
            wrap.appendChild(input);
            const hint = document.createElement("span");
            hint.className = "field__hint";
            hint.textContent =
                "Standard cron syntax (minute hour day month weekday).";
            wrap.appendChild(hint);
            return wrap;
        }

        function appendGeoFenceFields(body, rule) {
            body.appendChild(
                textField("Latitude key", rule, "LatKey", {
                    hint: "Context key for latitude.",
                }),
            );
            body.appendChild(
                textField("Longitude key", rule, "LngKey", {
                    hint: "Context key for longitude.",
                }),
            );

            const centerWrap = document.createElement("div");
            centerWrap.className = "field geo-field";
            const centerLbl = document.createElement("label");
            centerLbl.className = "field__label";
            centerLbl.textContent = "Center";
            const coords = document.createElement("div");
            coords.className = "geo-field__coords";

            const latInput = document.createElement("input");
            latInput.className = "input";
            latInput.type = "number";
            latInput.step = "any";
            latInput.placeholder = "Latitude";
            latInput.value = rule.LatCenter != null ? rule.LatCenter : "";

            const lngInput = document.createElement("input");
            lngInput.className = "input";
            lngInput.type = "number";
            lngInput.step = "any";
            lngInput.placeholder = "Longitude";
            lngInput.value = rule.LngCenter != null ? rule.LngCenter : "";

            function syncCenter() {
                rule.LatCenter = parseFloat(latInput.value) || 0;
                rule.LngCenter = parseFloat(lngInput.value) || 0;
                notifyChange();
            }
            latInput.addEventListener("input", syncCenter);
            lngInput.addEventListener("input", syncCenter);
            coords.appendChild(latInput);
            coords.appendChild(lngInput);
            centerWrap.appendChild(centerLbl);
            centerWrap.appendChild(coords);
            body.appendChild(centerWrap);

            const radiusWrap = document.createElement("div");
            radiusWrap.className = "field geo-field";
            const radiusLbl = document.createElement("label");
            radiusLbl.className = "field__label";
            radiusLbl.textContent = "Radius";
            const radiusRow = document.createElement("div");
            radiusRow.className = "geo-field__radius";

            const radiusInput = document.createElement("input");
            radiusInput.className = "input";
            radiusInput.type = "number";
            radiusInput.min = "0";
            radiusInput.step = "any";

            const unitSelect = document.createElement("select");
            unitSelect.className = "select";
            const units = [
                { label: "Meters", mult: 1 },
                { label: "Kilometers", mult: 1000 },
                { label: "Miles", mult: 1609.344 },
            ];
            units.forEach(function (unit) {
                const opt = document.createElement("option");
                opt.value = String(unit.mult);
                opt.textContent = unit.label;
                unitSelect.appendChild(opt);
            });

            const meters = rule.RadiusMeters != null ? rule.RadiusMeters : 0;
            let bestUnit = units[0];
            units.forEach(function (unit) {
                if (meters >= unit.mult) bestUnit = unit;
            });
            unitSelect.value = String(bestUnit.mult);
            radiusInput.value = String(
                Math.round((meters / bestUnit.mult) * 1000) / 1000,
            );

            function syncRadius() {
                const mult = parseFloat(unitSelect.value) || 1;
                rule.RadiusMeters = (parseFloat(radiusInput.value) || 0) * mult;
                notifyChange();
            }
            radiusInput.addEventListener("input", syncRadius);
            unitSelect.addEventListener("change", syncRadius);
            radiusRow.appendChild(radiusInput);
            radiusRow.appendChild(unitSelect);
            radiusWrap.appendChild(radiusLbl);
            radiusWrap.appendChild(radiusRow);
            body.appendChild(radiusWrap);
        }

        function rangeField(rule) {
            const wrap = document.createElement("div");
            wrap.className = "field range-field";

            const lbl = document.createElement("label");
            lbl.className = "field__label";
            lbl.textContent = "Bounds";

            const bounds = document.createElement("div");
            bounds.className = "range-field__bounds";

            function boundBlock(title, key, exclusiveKey) {
                const block = document.createElement("div");
                block.className = "range-field__bound";
                const head = document.createElement("div");
                head.className = "range-field__bound-head";
                head.textContent = title;
                const input = document.createElement("input");
                input.className = "input";
                input.type = "number";
                input.step = "any";
                input.value = rule[key] != null ? rule[key] : 0;
                input.addEventListener("input", function () {
                    rule[key] = parseFloat(input.value) || 0;
                    notifyChange();
                });
                const exLbl = document.createElement("label");
                exLbl.className = "checkbox";
                const exInput = document.createElement("input");
                exInput.type = "checkbox";
                exInput.checked = !!rule[exclusiveKey];
                exInput.addEventListener("change", function () {
                    rule[exclusiveKey] = exInput.checked;
                    notifyChange();
                });
                const exSpan = document.createElement("span");
                exSpan.textContent = "Exclude boundary";
                exLbl.appendChild(exInput);
                exLbl.appendChild(exSpan);
                block.appendChild(head);
                block.appendChild(input);
                block.appendChild(exLbl);
                return block;
            }

            bounds.appendChild(boundBlock("From", "Min", "ExclusiveMin"));
            bounds.appendChild(boundBlock("To", "Max", "ExclusiveMax"));
            wrap.appendChild(lbl);
            wrap.appendChild(bounds);
            return wrap;
        }

        const SEMVER_PRESETS = [
            ">= 1.0.0",
            ">= 2.0.0, < 3.0.0",
            "~1.2.0",
            "^1.0.0",
        ];

        function semVerConstraintField(rule) {
            const wrap = document.createElement("div");
            wrap.className = "field semver-field";

            const lbl = document.createElement("label");
            lbl.className = "field__label";
            lbl.textContent = "Constraint";

            const presets = document.createElement("div");
            presets.className = "semver-field__presets";

            const input = document.createElement("input");
            input.className = "input input--mono";
            input.type = "text";
            input.placeholder = ">= 1.2.0, < 2.0.0";
            input.value = rule.Constraint != null ? rule.Constraint : "";

            SEMVER_PRESETS.forEach(function (preset) {
                const btn = document.createElement("button");
                btn.type = "button";
                btn.className = "cron-field__preset";
                btn.textContent = preset;
                btn.addEventListener("click", function () {
                    input.value = preset;
                    rule.Constraint = preset;
                    notifyChange();
                });
                presets.appendChild(btn);
            });

            input.addEventListener("input", function () {
                rule.Constraint = input.value;
                notifyChange();
            });

            wrap.appendChild(lbl);
            wrap.appendChild(presets);
            wrap.appendChild(input);
            const hint = document.createElement("span");
            hint.className = "field__hint";
            hint.textContent =
                "Semver range compared against the context value.";
            wrap.appendChild(hint);
            return wrap;
        }

        function regexPatternField(rule) {
            const wrap = document.createElement("div");
            wrap.className = "field regex-field";

            const lbl = document.createElement("label");
            lbl.className = "field__label";
            lbl.textContent = "Pattern";

            const patternInput = document.createElement("input");
            patternInput.className = "input input--mono";
            patternInput.type = "text";
            patternInput.placeholder = "^prod-";
            patternInput.value =
                rule.RegexpPattern != null ? rule.RegexpPattern : "";

            const testWrap = document.createElement("div");
            testWrap.className = "regex-field__test";
            const sampleInput = document.createElement("input");
            sampleInput.className = "input input--mono";
            sampleInput.type = "text";
            sampleInput.placeholder = "Sample value to test";

            const status = document.createElement("span");
            status.className =
                "regex-field__status regex-field__status--nomatch";
            status.textContent = "No sample";

            function updateStatus() {
                rule.RegexpPattern = patternInput.value;
                notifyChange();
                const sample = sampleInput.value;
                if (!sample) {
                    status.textContent = "No sample";
                    status.className =
                        "regex-field__status regex-field__status--nomatch";
                    return;
                }
                if (!patternInput.value) {
                    status.textContent = "No pattern";
                    status.className =
                        "regex-field__status regex-field__status--nomatch";
                    return;
                }
                try {
                    const re = new RegExp(patternInput.value);
                    const matched = re.test(sample);
                    status.textContent = matched ? "Matches" : "No match";
                    status.className = matched
                        ? "regex-field__status regex-field__status--match"
                        : "regex-field__status regex-field__status--nomatch";
                } catch (e) {
                    status.textContent = "Invalid regex";
                    status.className =
                        "regex-field__status regex-field__status--invalid";
                }
            }

            patternInput.addEventListener("input", updateStatus);
            sampleInput.addEventListener("input", updateStatus);
            testWrap.appendChild(sampleInput);
            testWrap.appendChild(status);

            wrap.appendChild(lbl);
            wrap.appendChild(patternInput);
            wrap.appendChild(testWrap);
            const hint = document.createElement("span");
            hint.className = "field__hint";
            hint.textContent =
                "Try a sample value to confirm the pattern before saving.";
            wrap.appendChild(hint);
            return wrap;
        }

        function isSimpleValueData(v) {
            return (
                v === undefined ||
                v === null ||
                typeof v === "string" ||
                typeof v === "number" ||
                typeof v === "boolean"
            );
        }

        function valueDataField(label, obj, key) {
            const wrap = document.createElement("div");
            wrap.className = "field value-data-field";

            const labelRow = document.createElement("div");
            labelRow.className = "field__label-row";
            const lbl = document.createElement("label");
            lbl.className = "field__label";
            lbl.textContent = label;

            const groupName =
                "value-data-mode-" + Math.random().toString(36).slice(2);
            const modes = document.createElement("div");
            modes.className = "tester-source";
            modes.setAttribute("role", "radiogroup");
            modes.setAttribute("aria-label", "ValueData editor mode");

            function makeModeOption(value, text, checked) {
                const option = document.createElement("label");
                option.className = "tester-source__option";
                const input = document.createElement("input");
                input.type = "radio";
                input.name = groupName;
                input.value = value;
                input.checked = checked;
                const span = document.createElement("span");
                span.textContent = text;
                option.appendChild(input);
                option.appendChild(span);
                return { option: option, input: input };
            }

            let mode = isSimpleValueData(obj[key]) ? "simple" : "json";
            const simpleMode = makeModeOption(
                "simple",
                "Simple",
                mode === "simple",
            );
            const jsonMode = makeModeOption("json", "JSON", mode === "json");
            modes.appendChild(simpleMode.option);
            modes.appendChild(jsonMode.option);
            labelRow.appendChild(lbl);
            labelRow.appendChild(modes);
            wrap.appendChild(labelRow);

            const simpleWrap = document.createElement("div");
            simpleWrap.className = "value-data-field__simple";
            const simpleInput = document.createElement("input");
            simpleInput.className = "input input--mono";
            simpleInput.type = "text";
            simpleInput.placeholder = "e.g. on, true, 42";

            const ta = document.createElement("textarea");
            ta.className = "textarea textarea--mono";
            ta.spellcheck = false;
            ta.rows = 3;

            const err = document.createElement("span");
            err.className = "field__error";
            err.textContent = "Invalid JSON.";

            function setMode(next) {
                mode = next;
                simpleMode.input.checked = mode === "simple";
                jsonMode.input.checked = mode === "json";
                simpleWrap.hidden = mode !== "simple";
                ta.hidden = mode !== "json";
            }

            function switchToSimple() {
                if (mode === "simple") return;
                if (!ta.hidden) {
                    try {
                        if (ta.value.trim() !== "") {
                            obj[key] = JSON.parse(ta.value);
                        }
                    } catch (e) {
                        jsonMode.input.checked = true;
                        simpleMode.input.checked = false;
                        return;
                    }
                }
                loadFromObj();
                setMode("simple");
            }

            function switchToJson() {
                if (mode === "json") return;
                syncSimple();
                try {
                    ta.value = JSON.stringify(obj[key], null, 2);
                } catch (e) {
                    ta.value = "";
                }
                setMode("json");
            }

            function loadFromObj() {
                if (isSimpleValueData(obj[key])) {
                    simpleInput.value =
                        obj[key] === undefined || obj[key] === null
                            ? ""
                            : String(obj[key]);
                    try {
                        ta.value =
                            obj[key] === undefined
                                ? ""
                                : JSON.stringify(obj[key], null, 2);
                    } catch (e) {
                        ta.value = "";
                    }
                    setMode("simple");
                } else {
                    try {
                        ta.value = JSON.stringify(obj[key], null, 2);
                    } catch (e) {
                        ta.value = "";
                    }
                    setMode("json");
                }
            }

            function syncSimple() {
                const raw = simpleInput.value.trim();
                if (raw === "") {
                    delete obj[key];
                } else if (raw === "true") {
                    obj[key] = true;
                } else if (raw === "false") {
                    obj[key] = false;
                } else if (raw === "null") {
                    obj[key] = null;
                } else {
                    const n = Number(raw);
                    if (!isNaN(n) && String(n) === raw) obj[key] = n;
                    else obj[key] = raw;
                }
                wrap.classList.remove("is-invalid");
                notifyChange();
            }

            function syncJson() {
                if (ta.value.trim() === "") {
                    delete obj[key];
                    wrap.classList.remove("is-invalid");
                    notifyChange();
                    return;
                }
                try {
                    obj[key] = JSON.parse(ta.value);
                    wrap.classList.remove("is-invalid");
                    notifyChange();
                } catch (e) {
                    wrap.classList.add("is-invalid");
                }
            }

            modes.addEventListener("change", function (evt) {
                if (evt.target.value === "simple") switchToSimple();
                else if (evt.target.value === "json") switchToJson();
            });

            simpleInput.addEventListener("input", syncSimple);
            ta.addEventListener("input", syncJson);

            simpleWrap.appendChild(simpleInput);
            wrap.appendChild(simpleWrap);
            wrap.appendChild(ta);
            wrap.appendChild(err);
            const hint = document.createElement("span");
            hint.className = "field__hint";
            hint.textContent =
                "Optional payload returned when this rule matches. Leave empty if unused.";
            wrap.appendChild(hint);
            loadFromObj();
            return wrap;
        }

        function computedPill(value) {
            const wrap = document.createElement("div");
            wrap.className = "field";
            const lbl = document.createElement("label");
            lbl.className = "field__label";
            lbl.textContent = "Computed VariantID";
            const pill = document.createElement("div");
            pill.className = "computed-pill";
            pill.textContent = value || "None";
            wrap.appendChild(lbl);
            wrap.appendChild(pill);
            return { wrap: wrap, pill: pill };
        }

        // ----- Computed variant helpers -----

        function computeVariant(ruleData) {
            const k = Object.keys(ruleData)[0];
            const r = ruleData[k];
            switch (k) {
                case "andRule":
                    if (!r.Rules || !r.Rules.length) return "&()";
                    return "&(" + r.Rules.map(computeVariant).join("+") + ")";
                case "orRule":
                    if (!r.Rules || !r.Rules.length) return "|()";
                    return "|(" + r.Rules.map(computeVariant).join("+") + ")";
                case "notRule":
                    if (!r.Rule) return "!()";
                    return "!(" + computeVariant(r.Rule) + ")";
                default:
                    return r.VariantID || "";
            }
        }

        // ----- Rule list / element rendering -----

        function renderList(rulesArray, options) {
            options = options || {};
            const listEl = document.createElement("div");
            listEl.className =
                "rule-list" + (options.nested ? " rule-list--nested" : "");

            // Track the "Add rule" button so we can insert new rule elements
            // right before it without rebuilding the whole list (which would
            // wipe collapsed state, focus, and JSON-field validity).
            let addBtn = null;

            function makeRuleEl(ruleData, toplevelIndex) {
                return renderRule(ruleData, {
                    hideValueData: options.hideValueData,
                    hidePriority: options.hidePriority,
                    onVariantChange: options.onVariantChange,
                    toplevelIndex: toplevelIndex,
                    onRemove: function () {
                        const i = rulesArray.indexOf(ruleData);
                        if (i >= 0) rulesArray.splice(i, 1);
                        if (options.onVariantChange) options.onVariantChange();
                        notifyChange();
                    },
                });
            }

            if (rulesArray && rulesArray.length) {
                rulesArray.forEach(function (ruleData, idx) {
                    listEl.appendChild(
                        makeRuleEl(ruleData, options.nested ? undefined : idx),
                    );
                });
            }

            if (!options.disableAdd) {
                addBtn = buildAddRule(function (newRule) {
                    rulesArray.push(newRule);
                    const el = makeRuleEl(
                        newRule,
                        options.nested ? undefined : rulesArray.length - 1,
                    );
                    listEl.insertBefore(el, addBtn);
                    if (options.onVariantChange) options.onVariantChange();
                    notifyChange();
                    revealRule(el);
                });
                listEl.appendChild(addBtn);
            }

            return listEl;
        }

        function renderRule(ruleData, opts) {
            opts = opts || {};
            const ruleTypeKey = Object.keys(ruleData)[0];
            const rule = ruleData[ruleTypeKey];

            const wrapper = document.createElement("div");
            wrapper.className = "rule";
            wrapper.dataset.type = ruleTypeKey;
            wrapper.__ruleData = ruleData;
            wrapper.id = ensureRuleId(ruleData);
            if (
                opts.toplevelIndex !== undefined &&
                opts.toplevelIndex !== null
            ) {
                wrapper.dataset.toplevelIndex = String(opts.toplevelIndex);
            }

            // ---- Header ----
            const header = document.createElement("div");
            header.className = "rule__header";
            header.setAttribute("role", "button");
            header.setAttribute("tabindex", "0");
            header.setAttribute("aria-label", "Toggle rule details");

            function toggleCollapsed() {
                wrapper.classList.toggle("is-collapsed");
            }
            header.addEventListener("click", function (evt) {
                // Ignore clicks on interactive children (links, buttons, inputs, etc.)
                // so those elements handle the event themselves.
                if (
                    evt.target.closest(
                        "a, button, input, textarea, select, label",
                    )
                ) {
                    return;
                }
                toggleCollapsed();
            });
            header.addEventListener("keydown", function (evt) {
                if (evt.target !== header) return;
                if (evt.key === "Enter" || evt.key === " ") {
                    evt.preventDefault();
                    toggleCollapsed();
                }
            });

            const title = document.createElement("div");
            title.className = "rule__title";

            const collapseBtn = document.createElement("button");
            collapseBtn.type = "button";
            collapseBtn.className = "rule__collapse";
            collapseBtn.setAttribute("aria-label", "Toggle rule details");
            collapseBtn.innerHTML =
                '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"/></svg>';
            collapseBtn.addEventListener("click", function (evt) {
                evt.stopPropagation();
                toggleCollapsed();
            });

            const typeLink = document.createElement("a");
            typeLink.className = "rule__type-link";
            typeLink.href = DOC_BASE + "#" + ruleTypeKey.toLowerCase();
            typeLink.target = "_blank";
            typeLink.rel = "noopener";
            typeLink.title = "View documentation for " + ruleTypeKey;
            typeLink.textContent = ruleTypeKey;

            title.appendChild(collapseBtn);
            title.appendChild(typeLink);
            header.appendChild(title);

            const actions = document.createElement("div");
            actions.className = "rule__actions";
            const deleteBtn = document.createElement("button");
            deleteBtn.type = "button";
            deleteBtn.className = "btn btn--ghost btn--sm rule__delete";
            deleteBtn.setAttribute("aria-label", "Delete rule");
            deleteBtn.innerHTML =
                '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/><path d="M10 11v6"/><path d="M14 11v6"/><path d="M9 6V4a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2"/></svg>';
            deleteBtn.addEventListener("click", function () {
                wrapper.classList.add("is-leaving");
                setTimeout(function () {
                    wrapper.remove();
                    if (opts.onRemove) opts.onRemove();
                }, 160);
            });
            actions.appendChild(deleteBtn);
            header.appendChild(actions);
            wrapper.appendChild(header);

            // ---- Body ----
            const body = document.createElement("div");
            body.className = "rule__body";
            wrapper.appendChild(body);

            // Computed variant pill ref for composites
            let pillRef = null;
            function refreshPill() {
                if (pillRef)
                    pillRef.pill.textContent =
                        computeVariant(ruleData) || "None";
                scheduleTocRefresh();
                if (opts.onVariantChange) opts.onVariantChange();
            }

            // Per-type body fields
            switch (ruleTypeKey) {
                case "exactMatchRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(textField("KeyValue", rule, "KeyValue"));
                    break;
                case "regexRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(regexPatternField(rule));
                    break;
                case "existsRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    break;
                case "fractionalRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(
                        percentageField("Percentage", rule, "Percentage"),
                    );
                    break;
                case "rangeRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(rangeField(rule));
                    break;
                case "inListRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(
                        listField("Items", rule, "Items", {
                            parseItem: true,
                            placeholder: "Value (string, number, true, false)",
                            hint: "Enter each allowed value. Numbers and booleans are parsed automatically.",
                        }),
                    );
                    break;
                case "prefixRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(textField("Prefix", rule, "Prefix"));
                    break;
                case "suffixRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(textField("Suffix", rule, "Suffix"));
                    break;
                case "containsRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(textField("Substring", rule, "Substring"));
                    break;
                case "ipRangeRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(
                        listField("CIDRs", rule, "CIDRs", {
                            placeholder: "e.g. 10.0.0.0/8",
                            addLabel: "Add CIDR",
                            hint: "One CIDR block per row.",
                        }),
                    );
                    break;
                case "geoFenceRule":
                    appendGeoFenceFields(body, rule);
                    break;
                case "dateTimeRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(dateTimeField("After", rule, "After"));
                    body.appendChild(dateTimeField("Before", rule, "Before"));
                    break;
                case "semVerRule":
                    body.appendChild(textField("Key", rule, "Key"));
                    body.appendChild(semVerConstraintField(rule));
                    break;
                case "cronRule":
                    body.appendChild(
                        textField("Key", rule, "Key", {
                            hint: "Context key used to evaluate the cron window.",
                        }),
                    );
                    body.appendChild(cronSpecField(rule, "CronSpec"));
                    body.appendChild(
                        durationField("Duration", rule, "Duration"),
                    );
                    break;
                case "andRule":
                case "orRule": {
                    if (!Array.isArray(rule.Rules)) rule.Rules = [];
                    body.appendChild(
                        renderList(rule.Rules, {
                            nested: true,
                            hideValueData: true,
                            hidePriority: true,
                            onVariantChange: refreshPill,
                        }),
                    );
                    break;
                }
                case "notRule": {
                    const nestedWrap = document.createElement("div");
                    nestedWrap.className = "rule-list rule-list--nested";

                    function renderNot() {
                        nestedWrap.innerHTML = "";
                        if (rule.Rule) {
                            nestedWrap.appendChild(
                                renderRule(rule.Rule, {
                                    hideValueData: true,
                                    hidePriority: true,
                                    onVariantChange: refreshPill,
                                    onRemove: function () {
                                        rule.Rule = null;
                                        renderNot();
                                        refreshPill();
                                        notifyChange();
                                    },
                                }),
                            );
                        } else {
                            nestedWrap.appendChild(
                                buildAddRule(function (newRule) {
                                    rule.Rule = newRule;
                                    renderNot();
                                    refreshPill();
                                    notifyChange();
                                    revealRule(
                                        nestedWrap.querySelector(".rule"),
                                    );
                                }),
                            );
                        }
                    }
                    renderNot();
                    body.appendChild(nestedWrap);
                    break;
                }
                case "overrideRule":
                    // No type-specific fields; just VariantID + Priority + ValueData below.
                    break;
            }

            // Variant / computed pill
            if (COMPOSITE_TYPES.has(ruleTypeKey)) {
                pillRef = computedPill(computeVariant(ruleData));
                body.appendChild(pillRef.wrap);
            } else {
                body.appendChild(
                    textField("VariantID", rule, "VariantID", {
                        onVariantChange: function () {
                            if (opts.onVariantChange) opts.onVariantChange();
                        },
                    }),
                );
            }

            // Priority
            if (!opts.hidePriority) {
                body.appendChild(numberField("Priority", rule, "Priority"));
            }

            // ValueData (always available on composites; otherwise hidden on nested children)
            if (!opts.hideValueData || COMPOSITE_TYPES.has(ruleTypeKey)) {
                body.appendChild(
                    valueDataField("ValueData", rule, "ValueData"),
                );
            }

            return wrapper;
        }

        // ----- Add-rule popover -----

        function resetAddRuleMenu(wrap) {
            const menu = wrap.querySelector(".add-rule__menu");
            if (!menu) return;
            menu.style.position = "";
            menu.style.top = "";
            menu.style.left = "";
            menu.style.bottom = "";
            menu.style.visibility = "";
        }

        function detachAddRuleReposition(wrap) {
            const handler = wrap.__repositionHandler;
            if (!handler) return;
            window.removeEventListener("resize", handler, true);
            if (wrap.__scrollRoot) {
                wrap.__scrollRoot.removeEventListener("scroll", handler, true);
            }
            wrap.__repositionHandler = null;
            wrap.__scrollRoot = null;
        }

        function positionAddRuleMenu(wrap) {
            const btn = wrap.querySelector(".btn");
            const menu = wrap.querySelector(".add-rule__menu");
            if (!btn || !menu) return;

            const gap = 6;
            const pad = 8;

            // Fixed positioning escapes overflow:hidden on nested .rule cards.
            menu.style.position = "fixed";
            menu.style.left = "-9999px";
            menu.style.top = "0";
            menu.style.bottom = "auto";
            menu.style.visibility = "hidden";

            const btnRect = btn.getBoundingClientRect();
            const menuRect = menu.getBoundingClientRect();

            // Prefer opening upward; fall back to downward near the viewport top.
            let top = btnRect.top - menuRect.height - gap;
            if (top < pad) {
                top = btnRect.bottom + gap;
            }
            top = Math.max(
                pad,
                Math.min(top, window.innerHeight - menuRect.height - pad),
            );

            let left = btnRect.left;
            left = Math.max(
                pad,
                Math.min(left, window.innerWidth - menuRect.width - pad),
            );

            menu.style.top = top + "px";
            menu.style.left = left + "px";
            menu.style.visibility = "";
        }

        function attachAddRuleReposition(wrap) {
            const handler = function () {
                if (wrap.classList.contains("is-open")) {
                    positionAddRuleMenu(wrap);
                }
            };
            wrap.__repositionHandler = handler;
            window.addEventListener("resize", handler, true);
            const scrollRoot = document.querySelector(".rules-card__body");
            wrap.__scrollRoot = scrollRoot;
            if (scrollRoot) {
                scrollRoot.addEventListener("scroll", handler, true);
            }
        }

        function closeAddRule(wrap) {
            wrap.classList.remove("is-open");
            resetAddRuleMenu(wrap);
            detachAddRuleReposition(wrap);
            if (wrap.__onDocDown) {
                document.removeEventListener(
                    "mousedown",
                    wrap.__onDocDown,
                    true,
                );
                wrap.__onDocDown = null;
            }
            if (wrap.__onKey) {
                document.removeEventListener("keydown", wrap.__onKey, true);
                wrap.__onKey = null;
            }
        }

        function buildAddRule(onPick) {
            const wrap = document.createElement("div");
            wrap.className = "add-rule";

            const btn = document.createElement("button");
            btn.type = "button";
            btn.className = "btn";
            btn.innerHTML =
                '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 5v14"/><path d="M5 12h14"/></svg> Add rule';

            const menu = document.createElement("div");
            menu.className = "add-rule__menu";
            menu.setAttribute("role", "menu");

            const searchWrap = document.createElement("div");
            searchWrap.className = "add-rule__search";
            const searchInput = document.createElement("input");
            searchInput.className = "input add-rule__search-input";
            searchInput.type = "search";
            searchInput.placeholder = "Search rule types…";
            searchInput.autocomplete = "off";
            searchInput.setAttribute("aria-label", "Search rule types");
            searchWrap.appendChild(searchInput);

            const listEl = document.createElement("div");
            listEl.className = "add-rule__list";

            const emptyEl = document.createElement("p");
            emptyEl.className = "add-rule__empty";
            emptyEl.hidden = true;
            emptyEl.textContent = "No rule types match your search.";

            RULE_TYPES.forEach((group) => {
                const groupEl = document.createElement("div");
                groupEl.className = "add-rule__group";
                const label = document.createElement("div");
                label.className = "add-rule__group-label";
                label.textContent = group.label;
                groupEl.appendChild(label);
                group.options.forEach((opt) => {
                    const optBtn = document.createElement("button");
                    optBtn.type = "button";
                    optBtn.className = "add-rule__option";
                    optBtn.setAttribute("role", "menuitem");
                    optBtn.innerHTML =
                        "<strong>" +
                        opt.type +
                        "</strong>" +
                        (opt.desc ? "<small>" + opt.desc + "</small>" : "");
                    optBtn.addEventListener("click", function () {
                        const key = toJsonKey(opt.type);
                        const newRule = {};
                        newRule[key] = {};
                        if (opt.type === "AndRule" || opt.type === "OrRule") {
                            newRule[key].Rules = [];
                        } else if (opt.type === "NotRule") {
                            newRule[key].Rule = null;
                        }
                        close();
                        onPick(newRule);
                    });
                    groupEl.appendChild(optBtn);
                });
                listEl.appendChild(groupEl);
            });

            menu.appendChild(searchWrap);
            menu.appendChild(listEl);
            menu.appendChild(emptyEl);

            function applyAddRuleSearch(query) {
                const q = query.trim().toLowerCase();
                let visibleCount = 0;
                listEl
                    .querySelectorAll(".add-rule__group")
                    .forEach(function (group) {
                        const groupLabel = group.querySelector(
                            ".add-rule__group-label",
                        );
                        const groupText = groupLabel
                            ? groupLabel.textContent.toLowerCase()
                            : "";
                        let groupVisible = false;
                        group
                            .querySelectorAll(".add-rule__option")
                            .forEach(function (opt) {
                                const text = opt.textContent.toLowerCase();
                                const show =
                                    !q ||
                                    text.includes(q) ||
                                    groupText.includes(q);
                                opt.hidden = !show;
                                if (show) {
                                    groupVisible = true;
                                    visibleCount++;
                                }
                            });
                        group.hidden = !groupVisible;
                    });
                emptyEl.hidden = visibleCount > 0;
                requestAnimationFrame(function () {
                    positionAddRuleMenu(wrap);
                });
            }

            searchInput.addEventListener("input", function () {
                applyAddRuleSearch(searchInput.value);
            });
            searchInput.addEventListener("click", function (evt) {
                evt.stopPropagation();
            });
            searchInput.addEventListener("keydown", function (evt) {
                evt.stopPropagation();
                if (evt.key === "Escape") {
                    evt.preventDefault();
                    close();
                }
            });

            wrap.appendChild(btn);
            wrap.appendChild(menu);

            function close() {
                searchInput.value = "";
                applyAddRuleSearch("");
                closeAddRule(wrap);
            }
            function open() {
                // Close any other open popovers first.
                document
                    .querySelectorAll(".add-rule.is-open")
                    .forEach(function (el) {
                        if (el !== wrap) closeAddRule(el);
                    });
                wrap.classList.add("is-open");
                searchInput.value = "";
                applyAddRuleSearch("");
                requestAnimationFrame(function () {
                    positionAddRuleMenu(wrap);
                    searchInput.focus();
                });
                attachAddRuleReposition(wrap);
                wrap.__onDocDown = onDocDown;
                wrap.__onKey = onKey;
                document.addEventListener("mousedown", onDocDown, true);
                document.addEventListener("keydown", onKey, true);
            }
            function onDocDown(evt) {
                if (!wrap.contains(evt.target)) close();
            }
            function onKey(evt) {
                if (evt.key === "Escape") close();
            }

            btn.addEventListener("click", function (evt) {
                evt.stopPropagation();
                if (wrap.classList.contains("is-open")) close();
                else open();
            });

            return wrap;
        }

        // ----- Initial render -----
        container.appendChild(renderList(state, { nested: false }));
        refreshRuleTOC();

        // ----- Collapse all / Expand all -----
        function setAllCollapsed(collapsed) {
            container.querySelectorAll(".rule").forEach((el) => {
                el.classList.toggle("is-collapsed", collapsed);
            });
        }
        document
            .querySelectorAll("[data-rules-collapse-all]")
            .forEach((btn) => {
                btn.addEventListener("click", function () {
                    setAllCollapsed(true);
                });
            });
        document.querySelectorAll("[data-rules-expand-all]").forEach((btn) => {
            btn.addEventListener("click", function () {
                setAllCollapsed(false);
            });
        });

        const rulesSearchBox = document.getElementById("rules-search-box");
        if (rulesSearchBox) {
            rulesSearchBox.addEventListener("input", applyRuleSearch);
        }
    }

    /* ============================================================
       Boot
       ============================================================ */

    function boot() {
        wireAllConfirmButtons(document);
        wireAllFlagRows(document);
        setupListSearch();
        setupNewFlagDialog();
        setupCategoryCombobox();
        setupJsonFieldBlocks();
        setupFormDirtyGuard();
        setupRuleBuilder();
        setupTester();
        wireTestResultLinks(document);
    }

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", boot);
    } else {
        boot();
    }

    // Re-wire confirm buttons on any new htmx-swapped content.
    document.addEventListener("htmx:afterSwap", function (evt) {
        if (evt.detail && evt.detail.target) {
            wireAllConfirmButtons(evt.detail.target);
            wireAllFlagRows(evt.detail.target);
            wireTestResultLinks(evt.detail.target);
        }
    });
})();
