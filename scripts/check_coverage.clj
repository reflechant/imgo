#!/usr/bin/env bb

(require '[babashka.fs :as fs]
         '[babashka.process :refer [shell]])

(def thresholds
  {"pkg/transpiler" 100.0
   "pkg/persistent" 100.0})

(def paths
  (keys thresholds))

(def module-name
  (->
   (shell
    {:out :string}
    "go list -m")
   :out
   str/trim-newline))

(def coverage-raw
  (->
   (shell
    {:out :string}
    "go tool cover -func=coverage.out")
   :out
   str/split-lines
   drop-last))

(defn percent
  "return coverage percent from line as a float"
  [line]
  (let [percent-raw (last (str/split line #"\s"))]
    (parse-double (subs percent-raw 0 (dec (count percent-raw))))))

(defn path-or-nil
  [[line path]]
  (let [full-prefix (str module-name "/" path)]
    (if (str/starts-with? line full-prefix)
      path
      nil)))

(defn path
  "returns the package/file this line matches with or nil"
  [line]
  (->>
   (for [path paths] [line path])
   (some path-or-nil)))

(def coverage-map
  (->>
   (group-by path coverage-raw)
   (filter (comp some? first))
   (map
    (fn [[path lines]]
      (let [percents (map percent lines)
            cnt (count percents)
            summa (reduce + 0 percents)
            avg (/ summa cnt)]
        [path (float avg)])))
   (into {})))

(def result
  (reduce
   (fn [result-map [path threshold]]
     (assoc result-map path {:expected threshold
                             :actual (coverage-map path)}))
   {}
   thresholds))

(def all-passed?
  (reduce
   (fn [acc [path {:keys [expected actual]}]]
     (let [actual (or actual 0.0)
           pass? (>= actual expected)
           tick (if pass? "\u001b[32m✓\u001b[0m" "\u001b[31m✗\u001b[0m")]
       (println (format "%s - expected coverage: %.1f%%, actual: %.1f%% %s" path expected actual tick))
       (and acc pass?)))
   true
   result))

(when-not all-passed?
  (System/exit 1))
