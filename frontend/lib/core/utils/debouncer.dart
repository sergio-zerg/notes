import 'dart:async';

/// Waits until a burst of calls has stopped before running the last one.
///
/// Search and other request-triggering input need this: the server rate limits
/// per client address, so one request per keystroke spends the budget on
/// intermediate states the user never meant to submit.
class Debouncer {
  Debouncer(this.duration);

  /// How long to wait after the final call.
  final Duration duration;

  Timer? _timer;

  /// Schedules [action], cancelling any call still pending from an earlier one.
  ///
  /// Only the most recent action survives, which is what makes the last
  /// keystroke the one that is sent.
  void run(void Function() action) {
    _timer?.cancel();
    _timer = Timer(duration, action);
  }

  /// Drops a pending call, if any.
  void cancel() => _timer?.cancel();

  /// Releases the timer. Required wherever the owner is disposed, because an
  /// uncancelled timer outlives the widget and fires against a dead element.
  void dispose() {
    _timer?.cancel();
    _timer = null;
  }
}
