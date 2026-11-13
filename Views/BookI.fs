module Views.BookI

open Oxpecker
open Oxpecker.ViewEngine
open Views.Misc
open Views.Page

let index : EndpointHandler =
    frame (
        // Article markup is semantic; prose is based on rawcontent/Book-I.txt.
        main(class'="article-page") {
            article(class'="book-article") {
                // Reuse the same navigation component before and after the text.
                Back.view
                header(class'="article-header") {
                    h1() { "Book I" }
                    p(class'="article-deck") { "On cause." }
                }

                section() {
                    p() { "Cause and effect follow each other necessarily. The difference between the two is not ontic, but epistemic. What is a cause is an effect of the cause prior; inversely, what is an effect, is a cause of the effect ex post. The process of effectuation given a prior set of causes is Causal Succession." }
                }

                section() {
                    h2() { "Time" }
                    p() { "Time is a construct to express an index in Causal Succession. Time herein is not relativistic, but a point in the chain of Causal Succession." }
                }

                section() {
                    h2() { "Substance, Cosmos and Substrate" }
                    p() { "The Substance is that which can physically instantiate, preserve and transform state, and causally interact with other Substances through lawful alteration. It is the imagined irreducible unit of existence. Matter and Energy are different modes of Substance. Energy can instantiate physically. Matter can transform into Energy. Vacuum is not non-existence, but a mode of Substance. It is because vacuum can lawfully interact with other Substances." }
                    p() { "The totality of all Substances in the state of existence is the Cosmos. The Substance is the indivisible unit of it." }
                    p() { "A Substrate is a subset of the Cosmos. It consists only of the Substance. Different configurations of the Substance produce essential properties of the Substrate. Every physical matter is a certain configuration of the Substance. All energies are also Substance in different configurations. Each of such configurations is a Mode of Substance." }
                    p() { "Substance cannot emerge from non-existence. Substance cannot become non-existent. Creation and destruction are possible of a certain Modes of Substance but not Substance itself." }
                }

                section() {
                    h2() { "State of Existence" }
                    p() { "A state of Existence is the total description of the totality of Cosmos." }
                }

                section() {
                    h2() { "Causal Succession" }
                    p() { "Causal Succession is a continuous traversal of one state of existence to the next. It is an expression of monotonic transformation of at least one substrate in the entire state of existence." }
                    p() { "Such transformation is either change of mode of a Substance or a relative position of a Substance. In either scenario, the total description of the Cosmos differs." }
                    p() { "It cannot be begun and cannot be ended. It cannot be begun because, the very cause that begins effectuation must be an effect of a cause prior, to posit an uncaused cause is special pleading. Same applies for ending the Causal Succession." }
                }

                section() {
                    h2() { "Logic" }
                    p() { "The total prior state of existence admits only one successor state. The laws by which causal succession effectuates the successor state are called Logic. Randomness and probability arise when an observer is limited by the epistemic boundary of knowledge. Logic determines a singular outcome, not probabilistic." }
                }

                section() {
                    h2() { "Proposition" }
                    p() { "A Proposition is a candidate ontic record of a successor state of existence. Not all Propositions can effectuate; a Proposition that cannot effectuate is not a failed proposition but a malformed candidate. The capacity to effectuate is a property of the Proposition's structure, not of causal succession's arbitration." }
                }

                section() {
                    h2() { "Cognition and Agent" }
                    p() { "The capacity to derive a Proposition is Cognition. A substrate capable of Cognition is an Agent. Agency emerges at Cognition and scales with the ratio of coherence and effectuation of the propositions an Agent produces." }
                }

                section() {
                    h2() { "Determinism and Coherence" }
                    p() { "Logic is singular because existence itself is singular. A coherent proposition must derive from lawful consistency terminating at complete tautology. Contradiction cannot effectuate; therefore, incompatible successor states cannot coexist ontically." }
                    p() { "For any total prior state of existence, only one successor state can remain fully coherent with the complete lawful structure of causal succession. Multiple candidate successor states may exist epistemically due to incomplete access to the total state of existence, but such plurality reflects limitation of epistemic knowledge rather than plurality in ontic succession itself." }
                    p() { "Thus, determinism is not imposed upon existence externally, but emerges necessarily from the singular coherence of Logic. Randomness, probability and modal branching are epistemic artifacts arising from insufficient knowledge of the total prior state." }
                    p() { "A Proposition is coherent insofar as its derivation remains consistent with the total lawful structure of existence. A proposition effectuates only when such coherence corresponds perfectly with the successor state produced in causal succession." }
                }

                section() {
                    h2() { "Axiomata" }
                    h3() { "Axiom I: Law of Consistency" }
                    p() { "At any given state of Existence, the novel description of the entire Cosmos must be logically consistent to prior descriptions." }
                    p() { "The successor is the unique state that satisfies all logical necessities arising from every prior resolution. This is a logical conjunction: the successor effectuates only if it is consistent with the entire chain of necessities." }
                    h3() { "Axiom II: Law of Preservation" }
                    p() { "No Substrate enters or exits the State of Existence between Causal Successions. No Substance can be created and no Substance may cease to exist." }
                    h3() { "Axiom III: Law of Total Dependency" }
                    p() { "The successor is determined by the entire prior state, not any epistemic subset. The asymmetry in epistemic subsets ground probability of effectuation." }
                }

                section() {
                    h2() { "Actor" }
                    p() { "An actor is a configuration of Substance capable of producing causal outputs. The boundary of such configuration is not ontic, but an epistemic abstraction used to isolate localized causal relations within Causal Succession. Such partition is reinforced by persistent causal distinction from the surrounding Substance configurations." }
                }

                section() {
                    h2() { "Cognition" }
                    p() { "Cognition is an output of an Actor that is capable of:" }
                    ul() {
                        li() { "deriving an incomplete Proposition of a successor state; and" }
                        li() { "constructing a model of a post-hoc incomplete State of Existence from a prior one." }
                    }
                    p() { "Ontically, an actor is incapable of governing its output. It only derives so on the basis of its own incomplete access to Logic and ontic records. The causal output of such an actor is called Action. However, within the subset of Logic and ontic records, self-governance of an Actor emerges." }
                }

                section() {
                    h2() { "Telos" }
                    p() { "Cognition is contingent on certain configurations in an Actor. A Persistent Actor is one whose causal outputs retain such configuration. Telos is a specific cognitive output within Persistent Actors that remains compatible with the aforementioned substantial capacity for Cognition. It emerges not as an ontic necessity, rather as a self-selection on Actors." }
                }

                section() {
                    h2() { "Asymmetry" }
                    p() { "Cognitive boundaries of Cognitive Actors arise from prior ontic non-uniformity in Substance configuration, which constrains accessible History under finite epistemic access. While all Action is determined by the total State of Existence, the internal derivation of Propositions within an Actor is conditioned by partial access and therefore produces epistemically non-unique interpretations over identical ontic conditions. Each Causal Succession preserves ontic determinacy while updating internal epistemic structure in a path-dependent manner, thereby amplifying asymmetry in subsequent derivations." }
                }

                section() {
                    h2() { "Self-Governance" }
                    p() { "Due to the lack of access to the complete ontic records and Logic, a cognitive actor generates propositions wherein it is capable of deciding its outcome. These Propositions are analytical optimizations that the cognitive actors perform, their internal configuration that generates their causal output compatible with retaining persistence." }
                }

                section(class'="article-ending") {
                    h2() { "TERMINVS" }
                }
                Back.view
            }
        }
    )
    |> htmlView
