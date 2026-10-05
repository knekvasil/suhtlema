# Headphones are mandatory for the MVP

Status: accepted

Until the Companion runs through a speaker (post-MVP, alongside the mobile client), the MVP requires headphones so the microphone does not hear the Companion's own output. Without echo cancellation the VAD would trigger on the Companion's own voice, making barge-in impossible or immediately false. Shipping AEC was judged too costly for the MVP. This keeps the microphone path clean and keeps barge-in timing measurable.
