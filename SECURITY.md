# Security policy

adocfmt currently has a single maintainer, so the reply time below is a target rather than a guarantee.

## Report a vulnerability

Report a vulnerability privately at https://github.com/ypfaff/adocfmt/security/advisories/new.
Do not open a public issue, pull request, or discussion for it.

Include as much of the following as you can:

- The affected version, as `adocfmt --version` prints it, or the commit.
- The steps or the input that reproduce the problem.
- What an attacker gains, and under which conditions.
- A fix or a workaround, if you have one.

## What happens next

1. You get a first reply within 7 days.
2. The maintainer confirms or declines the report and keeps you updated in the advisory.
3. A confirmed vulnerability is fixed, in a new release when the binary or the GitHub Action is affected.
   The advisory is published with that release and credits you, unless you prefer otherwise.

Disclosure is coordinated: the advisory stays private until the fixed release is out, for at most 90 days after your report.

## Supported versions

Only the latest release gets security fixes.
The fix ships as a new release.

## Formatting bugs

A formatting result that changes what your document shows is a bug, not a vulnerability; open an issue for it.
[How adocfmt keeps your document safe](https://ypfaff.github.io/adocfmt/explanation/safety.html) explains what adocfmt guarantees.
