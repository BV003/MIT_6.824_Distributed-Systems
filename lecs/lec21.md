# Lecture 21: Bitcoin (Decentralized Consensus & Proof-of-Work)

## 1. What is Bitcoin?
Introduced by Satoshi Nakamoto in 2008, Bitcoin is a peer-to-peer electronic cash system. It represents a landmark breakthrough in distributed systems: it maintains a secure, ordered, public transaction ledger across a **permissionless network** where participants are completely anonymous, untrusted, and some are actively corrupt.

---

## 2. The Core Problems & Technical Solutions

### A. The Double-Spending Problem
* **The Problem**: Unlike physical cash, digital transactions can be easily copied. If owner `Y` signs a transfer of coin `C` to `Z` (`Y -> Z`) and simultaneously signs a transfer of the same coin `C` to `Q` (`Y -> Q`), both transactions are cryptographically valid. 
* **The Solution**: A shared, public ledger called the **Blockchain**. Peers only honor a transaction if it is in the main blockchain and has not spent a coin already consumed by an earlier block.

### B. Prevention of Sybil Attacks (Proof-of-Work)
* **The Problem**: In a decentralized system, an attacker can create millions of virtual machines to dominate a traditional voting system (a Sybil attack).
* **The Solution**: Bitcoin uses **"one CPU/Hashrate, one vote"** instead of "one node, one vote." 
  * To append a block of transactions to the blockchain, a node must solve a difficult cryptographic puzzle: finding a `nonce` value such that the hash of the block has $N$ leading zeros.
  * This **Proof-of-Work (PoW)** requires real-world physical electricity and computing power, making it extremely expensive to spam or manipulate.

### C. Resolving Forking (The Longest Chain Rule)
* **The Problem**: Due to network latency, two honest miners might find valid blocks at the same time, causing the blockchain to temporarily split (fork).
* **The Solution**: Peers always mine on top of, and switch to, the **longest valid chain** (the chain with the most cumulative proof-of-work). Because block discovery is random with high variance, one branch of the fork will quickly become longer than the other, and all peers will naturally converge onto it, discarding the shorter orphan chain.

---

## 3. The 51% Attack
If an attacker controls more than 50% of the network's total hashing (computing) power:
1. They can secretly mine a private, alternative fork where they double-spend their coins.
2. Because they have the majority of the hashing power, their private fork will grow faster than the honest public chain.
3. When the attacker broadcasts their longer chain, the network's longest-chain rule forces all honest nodes to switch to it, wiping out the honest transactions.

---

## 4. Key Architectural Trade-offs & Weaknesses
* **Incentives**: Miners are incentivized to behave honestly and spend electricity through **block rewards** (newly minted bitcoins) and **transaction fees** included in the blocks they find.
* **Latency vs. Finality**: Transactions are not instantly final. A payee must wait for several blocks (typically 6 blocks, or 60 minutes) to be added on top of their transaction block to be highly confident that their transaction won't be replaced by a transient fork.
* **Throughput Limitations**: The block size limit (1 MB) and the hardcoded 10-minute average block time limit Bitcoin's global throughput to roughly **5 to 7 transactions per second** (compared to credit card networks which handle 5,000+ per second).
* **Environmental Impact**: PoW mining leads to an arms race in specialized ASIC hardware, consuming massive amounts of global electricity.
