# Team Values

## Working software is the measure

*Everything else serves this goal.*

First and foremost, we value working, secure software in the hands of users. Every practice, investment, and process decision we make serves that goal. The values below describe how we get there.

## Communicate in the open

*Kind while honest, assume good intent, no sacred cows.*

We default to public channels over DMs and shared documents over tribal knowledge. When we give feedback, we're direct but respectful — we challenge ideas, not people. We assume our teammates are acting with good intent, even when we disagree. No topic is off-limits for constructive questioning: past decisions, current architecture, and team processes are all fair game if revisiting them makes us better.

## Drive work end-to-end

*Don't stop at boundaries, own it across layers.*

When we pick up work, we own it from design through deployment and beyond. We don't throw code over the wall to another team or stop at the edge of our component. If delivering a feature means modifying an upstream dependency, writing an operational runbook, or updating monitoring, that's part of the work. We follow through until the outcome is achieved, not just until our part is "done."

## Unblock each other first

*Enable peers to succeed, review quickly, share knowledge.*

A teammate waiting on a review or an answer is a teammate who can't make progress. We treat unblocking others as higher priority than our own feature work, because throughput of the whole team matters more than any individual's velocity. This means timely code reviews, pairing when someone is stuck, documenting what we learn, and proactively sharing context rather than waiting to be asked.

## Celebrate each other's wins

*Recognize contributions, make appreciation visible.*

Our culture recognizes and rewards the results that drive us forward. We take time to acknowledge each other's successes — big and small. When we make appreciation specific and visible, we support one another and reinforce the behaviors that lead to our collective success.

## Reliability is the most important feature

*Operations mindset, lean implementations, minimize maintenance surface.*

Users don't benefit from features that don't work. We think about failure modes, observability, and operational burden from the start — not as afterthoughts. We prefer simple, well-understood implementations over clever ones, and we actively reduce the surface area we have to maintain. Every line of code is a liability; we earn the complexity we carry.

We follow an upstream-first philosophy: we build what we need into existing codebases with larger user bases and community support, rather than maintaining our own. We minimize the repositories we own and collaborate with adjacent teams to share infrastructure rather than reinvent it.

## Run through two-way doors, deliberate before one-way doors

*Move fast on reversible choices, think carefully on irreversible ones.*

Not every decision carries the same weight. Two-way doors — choices that are cheap to reverse — should be made quickly, fearlessly, and learned from. One-way doors — choices that are expensive or impossible to undo, like public API contracts or customer-impacting changes — deserve careful deliberation, broader input, and written proposals. Recognizing which kind of decision we're facing keeps us both fast and safe.

## Solid foundations, fast iterations

*Get the fundamentals right, then ship small and learn.*

We invest in composable architecture, reliable CI, and clear operational practices — and we hold ourselves to a high bar on the foundations that make speed sustainable:

- **CI and testing** — Quick, reliable testing signal lets us respond fast to bugs and regressions. If CI is slow or flaky, we fix it before adding features.
- **Deployment pipeline** — Highly automated, from commit to production. Manual gates are the exception, not the rule.
- **Security posture** — We start from a "Zero Operator" philosophy and assume CVEs will arrive daily. Strong CI and automated deployment are our first line of defense.
- **AI-augmented engineering** — We invest in foundations today for AI to hook into our processes — from backlog refinement to development, testing, deployment, and operations. This lets us iterate quickly on agent quality and compound our velocity over time.

We eagerly adopt good components from elsewhere — if an off-the-shelf solution fits, that's one less thing to maintain and a sign it's not where we differentiate. When something almost fits, we contribute upstream rather than fork or rewrite. We save our complexity budget for the problems only we can solve. Strong foundations make fast iteration sustainable; fast iteration validates that our foundations are right.
