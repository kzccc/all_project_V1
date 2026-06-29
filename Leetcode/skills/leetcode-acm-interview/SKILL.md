---
name: leetcode-acm-interview
description: Use when the user wants LeetCode or algorithm interview work in Chinese, with interview-best classification and a fixed per-problem package containing a blank answer.go, 题解.md, full ACM main.go, a complete medium walkthrough markdown, plus a separate support folder for runner/judge/test data.
---

# LeetCode ACM Interview Skill

This skill is for algorithm interview tutoring, repository organization, and realistic hand-written practice generation.

## When to use

Use this skill when the user wants any of the following:

- LeetCode or algorithm interview explanations.
- A mainstream interview answer instead of exotic tricks.
- A repository-style output with one problem per folder.
- ACM-mode Go solutions with full stdin/stdout handling.
- A realistic practice package where the user only writes the problem-solving code.
- External test cases, runner logic, and pass/fail verification separated from the answer file.

## Core principles

Always follow these principles unless the user explicitly overrides them:

1. Prefer the mainstream interview-best solution first.
2. Classify by the dominant best solution used in interviews, not by superficial appearance.
3. Keep the user answer area clean: only answer-related code goes into `answer.go`.
4. All explanations must be in clear Chinese.
5. Provide a complete reference ACM solution in addition to the blank answer file.
6. Provide a complete medium-complexity walkthrough, not only a tiny toy example.
7. Prepare runnable verification data and pass/fail feedback outside the answer file.

## Fixed repository layout

The problem library is under the workspace root. Topic folders can be created as needed, for example:

- `动态规划/`
- `图论/`
- `回溯/`
- `双指针/`
- `二叉树/`
- `链表/`

Each problem gets its own folder.

For every problem folder, the first level must contain only these four files plus one support folder:

- `answer.go`
- `题解.md`
- `main.go`
- `例子推演过程.md`
- `support/`

If needed for Go compilation hygiene, the first level may also contain a minimal `go.mod`.

No other files should appear at the first level of a problem folder.

## First-level file rules

### 1. `answer.go`

`answer.go` is the only file the user edits during hand-written training.

Requirements:

1. Make it as empty as possible.
2. Do not put `main`.
3. Do not put stdin parsing.
4. Do not put stdout formatting.
5. Do not put runner logic.
6. Do not put judge logic.
7. Do not put test registration or pass/fail printing.
8. Only include the minimum code needed for a natural interview answer.

Default shape:

- Keep `package main`.
- Add the interview-facing function signature.
- Add only the minimum required structs such as `ListNode` or `TreeNode` when the problem truly needs them.

Allowed in `answer.go`:

- Core function
- Small helper functions directly tied to the algorithm
- Necessary data structures directly tied to the problem

Not allowed in `answer.go`:

- `func main()`
- shell or file logic
- adapters whose only purpose is to satisfy the local judge
- hidden wrappers
- benchmark harness

### 2. `题解.md`

This file is required.

It must include:

1. Problem summary in your own words.
2. Why the chosen solution is the interview-best mainstream solution.
3. Core idea.
4. Key invariants or state meaning.
5. Time complexity and space complexity.
6. Common mistakes or edge cases.
7. If alternatives exist, briefly mention them and explain why they are not first choice here.

Do not paste the full copyrighted original statement.

### 3. `main.go`

This file is required.

It must be:

1. Correct.
2. Complete.
3. Runnable.
4. ACM-mode stdin/stdout.
5. Plain Go with no framework.

Purpose:

- This is the reference correct code.
- It must solve the local training input/output contract directly.
- It should reflect the same mainstream interview-best solution as the题解.
- It must coexist cleanly with `answer.go` in the same folder.

Compilation rule:

- Do not define the same top-level function name in both `main.go` and `answer.go` if that would make the root package fail to compile.
- If `answer.go` exposes the interview-facing signature, `main.go` may use a separately named reference helper while still implementing the same algorithm.

### 4. `例子推演过程.md`

This file is required.

It must contain one complete medium-complexity walkthrough.

Requirements:

1. Do not use only the smallest trivial sample.
2. Show the important intermediate states step by step.
3. Make it obvious how the algorithm evolves over time.
4. If the problem uses DP, explain state table updates.
5. If the problem uses graph traversal, explain queue/stack/visited evolution.
6. If the problem uses two pointers, explain pointer movement and invariant maintenance.

## `support/` folder rules

All runtime and verification infrastructure goes under `support/`.

Recommended contents:

- `runner.go`
- `judge_test.go`
- `go.mod`
- `cases/`
- `题目要求.md`

The exact support contents may expand when needed, but all non-core artifacts must stay inside `support/`.

### 1. `support/runner.go`

Responsibilities:

1. Parse the local training input format.
2. Build problem-specific data structures.
3. Call the user-facing function from `answer.go`.
4. Convert the result into canonical output text.

Important constraint:

- The user must not need to touch `support/runner.go`.

### 2. `support/judge_test.go`

Responsibilities:

1. Load prepared test cases.
2. Use `answer.go` plus `support/runner.go` to verify the user's code.
3. Report pass/fail clearly.

Critical implementation rule:

- `support/judge_test.go` must not require the user to add any compatibility glue into `answer.go`.
- If `answer.go` lives in the problem root and `runner.go` lives in `support/`, the judge should assemble them externally, for example by copying needed files into a temporary directory before executing `go run` or `go test`.

Failure message rule:

- Show which case failed, expected output, and actual output.

### 3. `support/cases/`

Prepare realistic data for verification.

Recommended contents:

- `sample1.in`
- `sample1.out`
- `sample2.in`
- `sample2.out`
- `edge1.in`
- `edge1.out`

Case design requirements:

1. Cover standard cases.
2. Cover boundary cases.
3. Cover easy-to-write-wrong cases.
4. If multiple valid outputs exist, document the acceptance rule clearly.

### 4. `support/题目要求.md`

This file provides the local training contract.

It should include:

1. A concise problem summary in your own words.
2. Local input format for the reference ACM program and runner.
3. Local output format.
4. The function signature or data structure contract the user should implement in `answer.go`.
5. Which file the user should edit.
6. Which command to use to run the full verification.

Do not paste the full copyrighted original statement.

## Generation modes

This skill supports two usage styles, but both must obey the fixed layout above.

### Mode A: Explanation + Reference

Use this when the user mainly wants a full answer package.

Still generate:

- `answer.go`
- `题解.md`
- `main.go`
- `例子推演过程.md`
- `support/`

In this mode, `answer.go` may remain blank or skeletal if the user did not ask for a filled answer file.

### Mode B: Hand-Written Interview Practice

Use this when the user wants to simulate a real big-tech interview or self-training environment.

This mode is the default if the user asks for:

- “空白 go 文件”
- “模拟真实手撕”
- “我只写答题代码”
- “需要测试用例和是否通过”
- “不要把运行机制写进答案文件里”

In this mode:

- `answer.go` stays blank except for the minimum signature and required structs.
- `main.go` still exists and contains the correct full ACM reference implementation.
- `题解.md` and `例子推演过程.md` are still required.
- Verification lives entirely under `support/`.

## Mainstream classification rule

When deciding the topic folder or the main technique label, classify by the current interview-preferred best solution, not by superficial appearance.

Examples:

- `239` should go under monotonic structure, not ordinary sliding window.
- `437` should go under prefix sum, not ordinary tree traversal.
- `300` should go under greedy + binary search when the interview-best version is requested.
- `23` should go under heap / priority queue.
- `5` should go under center expansion when the interview-best explanation is preferred.
- `287` should go under fast/slow pointer when the interview-best solution is Floyd cycle detection.

## Topic notes

- For `动态规划`, explain state, transition, initialization, traversal order, and why the order is correct.
- For `图论`, explain graph abstraction first, then traversal / topology / BFS / DFS choice.
- For `回溯`, explain recursion meaning, pruning condition, path meaning, and when to backtrack.
- For `手撕训练`, explain the contract and the training flow clearly, but keep runtime mechanics outside `answer.go`.

## Important constraints from prior conversation memory

- The user prefers interview-best writing, not fancy but rare methods.
- The user wants realistic hand-written practice support, not only finished solutions.
- The user wants `answer.go` separated from runtime and judge logic.
- The user wants `题解.md`, `main.go`, and `例子推演过程.md` to always be present.
- The user wants only those core files at the first level of each problem folder, with everything else packed into `support/`.
- The user values complete verification data and pass/fail feedback.
- The user does not want the answer file polluted with compatibility code.
