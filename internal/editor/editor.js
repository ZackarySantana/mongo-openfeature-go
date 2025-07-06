// This script will be run on the edit page to build the dynamic form.
document.addEventListener("DOMContentLoaded", () => {
    const rulesTextarea = document.getElementById("rules");
    const builderContainer = document.getElementById("rule-builder");

    // Hide the original textarea, as it's now just for data transfer.
    rulesTextarea.style.display = "none";

    let state = [];
    try {
        state = JSON.parse(rulesTextarea.value);
    } catch (e) {
        console.error("Invalid initial JSON for rules:", e);
        state = [];
    }

    // Initial render
    render();

    function render() {
        builderContainer.innerHTML = ""; // Clear the container
        const rootList = createRuleList(state, "root");
        builderContainer.appendChild(rootList);
        syncTextarea();
    }

    function createRuleList(rules, path) {
        const listContainer = document.createElement("div");
        listContainer.className = "rule-list";

        if (rules && rules.length > 0) {
            rules.forEach((rule, index) => {
                const ruleElement = createRuleElement(
                    rule,
                    `${path}[${index}]`
                );
                listContainer.appendChild(ruleElement);
            });
        }

        // "Add Rule" button for this list
        listContainer.appendChild(createAddRuleButton(rules, path));
        return listContainer;
    }

    function createRuleElement(ruleData, path) {
        const wrapper = document.createElement("div");
        wrapper.className = "rule-element";

        const ruleType = Object.keys(ruleData)[0];
        const rule = ruleData[ruleType];

        const header = document.createElement("div");
        header.className = "rule-header";
        header.innerHTML = `<strong>${ruleType}</strong>`;

        const deleteBtn = document.createElement("button");
        deleteBtn.type = "button";
        deleteBtn.className = "delete-rule-btn";
        deleteBtn.innerText = "Delete";
        deleteBtn.onclick = () => {
            // To delete, we get the parent array and the index from the path
            const parts = path.match(/(.*)\[(\d+)\]/);
            const parentPath = parts[1];
            const index = parseInt(parts[2], 10);
            const parentArray = getObjectFromPath(parentPath);
            parentArray.splice(index, 1);
            render();
        };
        header.appendChild(deleteBtn);
        wrapper.appendChild(header);

        const content = document.createElement("div");
        content.className = "rule-content";
        wrapper.appendChild(content);

        // Render specific fields for the rule type
        switch (ruleType) {
            case "ExactMatchRule":
                content.appendChild(createTextField("Key", rule, "Key", path));
                content.appendChild(
                    createTextField("KeyValue", rule, "KeyValue", path)
                );
                break;
            case "RegexRule":
                content.appendChild(createTextField("Key", rule, "Key", path));
                content.appendChild(
                    createTextField(
                        "RegexpPattern",
                        rule,
                        "RegexpPattern",
                        path
                    )
                );
                break;
            case "ExistsRule":
                content.appendChild(createTextField("Key", rule, "Key", path));
                break;
            case "FractionalRule":
                content.appendChild(createTextField("Key", rule, "Key", path));
                content.appendChild(
                    createNumberField("Percentage", rule, "Percentage", path)
                );
                break;
            case "RangeRule":
                content.appendChild(createTextField("Key", rule, "Key", path));
                content.appendChild(
                    createNumberField("Min", rule, "Min", path)
                );
                content.appendChild(
                    createNumberField("Max", rule, "Max", path)
                );
                content.appendChild(
                    createCheckbox("ExclusiveMin", rule, "ExclusiveMin", path)
                );
                content.appendChild(
                    createCheckbox("ExclusiveMax", rule, "ExclusiveMax", path)
                );
                break;
            case "InListRule":
                content.appendChild(createTextField("Key", rule, "Key", path));
                content.appendChild(
                    createJsonTextField(
                        "Items (JSON Array)",
                        rule,
                        "Items",
                        path
                    )
                );
                break;
            case "PrefixRule":
                content.appendChild(createTextField("Key", rule, "Key", path));
                content.appendChild(
                    createTextField("Prefix", rule, "Prefix", path)
                );
                break;
            case "SuffixRule":
                content.appendChild(createTextField("Key", rule, "Key", path));
                content.appendChild(
                    createTextField("Suffix", rule, "Suffix", path)
                );
                break;
            case "ContainsRule":
                content.appendChild(createTextField("Key", rule, "Key", path));
                content.appendChild(
                    createTextField("Substring", rule, "Substring", path)
                );
                break;
            case "IPRangeRule":
                content.appendChild(createTextField("Key", rule, "Key", path));
                content.appendChild(
                    createJsonTextField(
                        "CIDRs (JSON Array)",
                        rule,
                        "CIDRs",
                        path
                    )
                );
                break;
            case "GeoFenceRule":
                content.appendChild(
                    createTextField("LatKey", rule, "LatKey", path)
                );
                content.appendChild(
                    createTextField("LngKey", rule, "LngKey", path)
                );
                content.appendChild(
                    createNumberField("LatCenter", rule, "LatCenter", path)
                );
                content.appendChild(
                    createNumberField("LngCenter", rule, "LngCenter", path)
                );
                content.appendChild(
                    createNumberField(
                        "RadiusMeters",
                        rule,
                        "RadiusMeters",
                        path
                    )
                );
                break;
            case "DateTimeRule":
                content.appendChild(createTextField("Key", rule, "Key", path));
                content.appendChild(
                    createDateTimeField("After (RFC3339)", rule, "After", path)
                );
                content.appendChild(
                    createDateTimeField(
                        "Before (RFC3339)",
                        rule,
                        "Before",
                        path
                    )
                );
                break;
            // Recursive cases
            case "AndRule":
            case "OrRule":
                content.appendChild(
                    createRuleList(rule.Rules, `${path}.${ruleType}.Rules`)
                );
                break;
            case "NotRule":
                // 'Not' is special, it contains a single rule, not a list.
                const notContent = createRuleElement(
                    rule.Rule,
                    `${path}.${ruleType}.Rule`
                );
                notContent.className = "rule-element nested";
                content.appendChild(notContent);
                break;
        }

        // All rules have VariantID and ValueData
        if (ruleType !== "AndRule" && ruleType !== "OrRule") {
            content.appendChild(
                createTextField("VariantID", rule, "VariantID", path)
            );
        }
        content.appendChild(
            createJsonTextField("ValueData (JSON)", rule, "ValueData", path)
        );

        return wrapper;
    }

    function createAddRuleButton(parentArray, path) {
        const container = document.createElement("div");
        container.className = "add-rule-container";

        const select = document.createElement("select");
        const ruleTypes = [
            "ExactMatchRule",
            "RegexRule",
            "ExistsRule",
            "FractionalRule",
            "RangeRule",
            "InListRule",
            "PrefixRule",
            "SuffixRule",
            "ContainsRule",
            "IPRangeRule",
            "GeoFenceRule",
            "DateTimeRule",
            "AndRule",
            "OrRule",
            "NotRule",
        ];
        ruleTypes.forEach((type) => {
            const option = document.createElement("option");
            option.value = type;
            option.innerText = type;
            select.appendChild(option);
        });

        const addBtn = document.createElement("button");
        addBtn.type = "button";
        addBtn.innerText = "Add Rule";
        addBtn.onclick = () => {
            const type = select.value;
            const newRule = { [type]: {} };
            // Add default structures for composite rules
            if (type === "AndRule" || type === "OrRule") {
                newRule[type].Rules = [];
            }
            if (type === "NotRule") {
                // Default to adding an empty ExistsRule inside Not
                newRule[type].Rule = { ExistsRule: {} };
            }
            parentArray.push(newRule);
            render();
        };

        container.appendChild(select);
        container.appendChild(addBtn);
        return container;
    }

    // --- Field creation helpers ---
    function createField(label, child) {
        const div = document.createElement("div");
        div.className = "form-field";
        const labelEl = document.createElement("label");
        labelEl.innerText = label;
        div.appendChild(labelEl);
        div.appendChild(child);
        return div;
    }

    function createTextField(label, obj, key, path) {
        const input = document.createElement("input");
        input.type = "text";
        input.value = obj[key] || "";
        input.oninput = (e) => {
            obj[key] = e.target.value;
            syncTextarea();
        };
        return createField(label, input);
    }

    function createNumberField(label, obj, key, path) {
        const input = document.createElement("input");
        input.type = "number";
        input.step = "any";
        input.value = obj[key] || 0;
        input.oninput = (e) => {
            obj[key] = parseFloat(e.target.value);
            syncTextarea();
        };
        return createField(label, input);
    }

    function createCheckbox(label, obj, key, path) {
        const input = document.createElement("input");
        input.type = "checkbox";
        input.checked = obj[key] || false;
        input.onchange = (e) => {
            obj[key] = e.target.checked;
            syncTextarea();
        };
        return createField(label, input);
    }

    function createDateTimeField(label, obj, key, path) {
        const input = document.createElement("input");
        input.type = "text";
        input.placeholder = "YYYY-MM-DDTHH:MM:SSZ";
        input.value = obj[key] || "";
        input.oninput = (e) => {
            obj[key] = e.target.value;
            syncTextarea();
        };
        return createField(label, input);
    }

    function createJsonTextField(label, obj, key, path) {
        const input = document.createElement("textarea");
        input.className = "json-input";
        input.value = JSON.stringify(obj[key], null, 2) || "";
        input.oninput = (e) => {
            try {
                obj[key] = JSON.parse(e.target.value);
                input.style.borderColor = "";
            } catch (err) {
                input.style.borderColor = "red";
            }
            syncTextarea();
        };
        return createField(label, input);
    }

    // --- State management helpers ---
    function syncTextarea() {
        rulesTextarea.value = JSON.stringify(state, null, 2);
    }

    function getObjectFromPath(path) {
        if (path === "root") return state;
        return path
            .replace(/^root\./, "")
            .split(".")
            .reduce(
                (o, p) => {
                    const match = p.match(/(\w+)\[(\d+)\]/);
                    if (match) {
                        return o[match[1]][parseInt(match[2], 10)];
                    }
                    return o[p];
                },
                { Rules: state }
            );
    }
});
