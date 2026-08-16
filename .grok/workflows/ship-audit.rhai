let meta = #{
    name: "ship-audit",
    description: "Fan-out production audit of whereToken CLI, adapters, dashboard, and install",
    phases: [
        #{ title: "Audit", detail: "four independent readers" },
        #{ title: "Verify", detail: "skeptics confirm or refute" },
    ],
    when_to_use: "When polishing whereToken toward a ship bar",
};

let findings_schema = #{
    "type": "object", "required": ["findings"],
    "properties": #{
        "findings": #{ "type": "array", "maxItems": 6, "items": #{
            "type": "object", "required": ["file", "issue"],
            "properties": #{
                "file": #{ "type": "string" },
                "issue": #{ "type": "string" },
            },
        }},
    },
};

let verdict_schema = #{
    "type": "object", "required": ["real", "reason", "evidence"],
    "properties": #{
        "real": #{ "type": "boolean" },
        "reason": #{ "type": "string" },
        "evidence": #{ "type": "string" },
    },
};

phase("Audit");
let jobs = [];
let dims = [
    "CLI help man JSON schema flags empty-state first-run",
    "scan adapters calendar conservation redaction",
    "Vue kiln dashboard glaze empty-state a11y serve headers",
    "install scripts formula goreleaser CI changelog README",
];
for d in dims {
    let p = "Read /Users/rainhuang/Desktop/whereToken with read_file and grep. ";
    p += "Find concrete production gaps about " + d + ". ";
    p += "Do not invent adapters. Do not decrypt Trae/Cursor. ";
    p += "Report at most 6 {file, issue} with file:line. Empty list only after reading code.";
    jobs.push(#{
        prompt: p,
        label: "audit",
        capability_mode: "read-only",
        output_schema: findings_schema,
    });
}
let results = parallel(jobs);
let findings = [];
for r in results {
    if r != () && r.success && r.output.findings != () {
        for f in r.output.findings { findings.push(f); }
    }
}
if findings.len() == 0 { complete(#{ summary: "No findings.", confirmed: [] }); }

phase("Verify");
let vjobs = [];
let i = 0;
while i < findings.len() {
    if i >= 8 { break; }
    let f = findings[i];
    let vp = "Adversarially verify by reading shipped code: \"";
    vp += f.issue + "\" in " + f.file + ". Set real=true only with evidence.";
    vjobs.push(#{
        prompt: vp,
        label: "verify",
        capability_mode: "read-only",
        output_schema: verdict_schema,
    });
    i += 1;
}
let verdicts = parallel(vjobs);
let confirmed = [];
let j = 0;
for v in verdicts {
    if v != () && v.success && v.output.real == true
        && v.output.evidence != () && v.output.evidence != "" {
        confirmed.push(findings[j]);
    }
    j += 1;
}
complete(#{ summary: confirmed.len().to_string() + " confirmed", confirmed: confirmed });
