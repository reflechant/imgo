#!/usr/bin/env bb

(require '[babashka.process :refer [shell]]
         '[clojure.string :as str])

(def thresholds
  {"pkg/transpiler" 100.0
   "pkg/persistent" 100.0})

(def module-name
  (-> (shell {:out :string} "go list -m")
      :out
      str/trim-newline))

(def coverage-lines
  (-> (shell {:out :string} "go tool cover -func=coverage.out")
      :out
      str/split-lines
      drop-last))

(defn parse-percent [line]
  (-> (re-find #"[\d.]+%" line)
      (str/replace "%" "")
      parse-double))

(defn package-of [line]
  (->> (keys thresholds)
       (filter #(str/starts-with? line (str module-name "/" %)))
       first))

(def coverage
  (->> coverage-lines
       (group-by package-of)
       (filter (comp some? key))
       (into {} (map (fn [[pkg lines]]
                       (let [percents (map parse-percent lines)]
                         [pkg (float (/ (reduce + percents) (count percents)))]))))))

(defn check [[pkg threshold]]
  (let [actual (get coverage pkg 0.0)
        pass?  (>= actual threshold)
        mark   (if pass? "[32m✓[0m" "[31m✗[0m")]
    (println (format "%s - expected: %.1f%%, actual: %.1f%% %s" pkg threshold actual mark))
    pass?))

(when-not (every? check thresholds)
  (System/exit 1))
