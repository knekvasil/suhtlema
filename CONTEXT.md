# Suhtlema

A spoken language companion: the Learner holds a conversation with the Companion in a target language, and the Companion answers aloud.

## Language

**Companion**:
The conversational agent the Learner speaks with.
_Avoid_: tutor, assistant, chatbot, bot

**Learner**:
The person practising a target language by talking with the Companion.
_Avoid_: user, student, client

**Turn**:
One Learner utterance together with the Companion's reply to it.
_Avoid_: exchange, round, message

**Barge-in**:
The Learner seizing the Turn from the Companion mid-reply, cancelling the Companion's current Turn.
_Avoid_: interruption (overloaded)
_Post-MVP._

**Aside**:
A meta-question the Learner asks mid-conversation (e.g. vocabulary or grammar) that the Companion answers briefly before the conversation resumes.
_Avoid_: mom mode, detour, tangent
_Post-MVP. Formerly called "mom mode"._

## Relationships

- A conversation is a sequence of **Turns**.
- A **Barge-in** cancels the Companion's current **Turn**; it does not begin a new topic.
- An **Aside** nests inside the current **Turn** and does not end the conversation thread.

## Example dialogue

> **Dev:** "If the Learner asks 'what's the word for receipt?' mid-sentence, is that a Barge-in?"
> **Domain expert:** "No. A Barge-in seizes the Turn. That's an Aside — it nests inside the thread and the thread resumes afterwards."

## Flagged ambiguities

- "mom mode" was used for the meta-question interaction; resolved: the canonical term is **Aside**, and it is not a **Barge-in**.
- "interruption" was used for both behaviours; resolved: **Barge-in** (turn seizure) and **Aside** (nested question) are distinct concepts.
