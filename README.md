RSH Synchronization for Unbound
===============================

If you're an ISP in Poland, you're likely required to blacklist domains from th
"Rejestr Stron Hazardowych niezgodnych z Ustawa". This is a small Go daemon that
should help you do that.

Usage
-----

    Usage of rsh-unbound:
      -alsologtostderr
        	log to standard error as well as files
      -log_backtrace_at value
        	when logging hits line file:N, emit a stack trace
      -log_dir string
        	If non-empty, write log files in this directory
      -logtostderr
        	log to standard error instead of files
      -output string
        	Path to generated Unbound config file (default "/etc/unbound/rsh.conf")
      -redirect string
        	Address to redirect to (default "145.237.235.240")
      -register_endpoint string
        	Address of RSH Registry endpoint (default "https://www.hazard.mf.gov.pl/api/Register")
      -stderrthreshold value
        	logs at or above this threshold go to stderr
      -v value
        	log level for V logs
      -vmodule value
        	comma-separated list of pattern=N settings for file-filtered logging

Configuration
-------------

    server:
        [...]
        include: "/etc/unbound/rsh.conf"

Todo
----

 - Use unbound-control instead of systemd to reload config.
 - Export metrics via Prometheus.

