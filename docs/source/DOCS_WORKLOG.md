- You are an experienced product manager and technical writer. You are rewriting documentation for a technical product from scratch. Create a list of problems that you need to solve in order to produce very high quality documentation that will maximize customer satisfaction and technical support/troubleshooting/integration problems so that it can be treated like a runbook for typical hurdles. Do not do anything else. Do not refer to particular technology, but be very precise about the deliverable in each of the points. Do not say or do anything else. Once done, keep looping (iterating) over your work to see what else could be improved, until the list is very complete, and I will be able to submit this work as done.
- solve 7
- expand 7: add the junior support engineer persona (somewhat experienced in scylladb, junior in k8s)
- solve 8
- solve 8 again, but instead of mapping existing journeys, invent new ones (content will follow)
- 8: manual edits 
- Review all open issues in the scylla-operator repository to understand what information is missing in the documentation. Add them under the "Content Completeness" section.
- Manual: Clean up the auto multi-DC confusion. Group Manual Multi-DC content gaps into a separate sub-section.
- Manual: extract the structure directives and the content directives into a section at the top of the hints file.
- You are an expert hands-on product manager and you are deeply dissatisfied with the quality of the existing product documentation.
Propose a structure (directories, index.md files and individual files) of the docs that breaks up with the existing docs structure. Follow the directives listed in DOCS_HINTS.md.
For each item attach a very concise directive about its content. Explicitly classify it in Diataxis. Be clear about what content should be taken from existing docs (link to file) and what new content will need to be written.
Prefer new content over currently existing content always if this way documentation addresses the persona needs better.

This structure will be a basis for a total overhaul of the existing documentation that will be deleted.