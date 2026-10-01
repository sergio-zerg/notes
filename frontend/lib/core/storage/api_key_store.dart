import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Persists the API key between launches.
///
/// The key is a shared secret equivalent to a password: anyone holding it can
/// read and modify every note. It is therefore kept in the platform keystore —
/// Keychain on iOS, EncryptedSharedPreferences on Android — and never in
/// `SharedPreferences` or a file that a device backup would carry in the clear.
class ApiKeyStore {
  ApiKeyStore({FlutterSecureStorage? storage})
    : _storage =
          storage ??
          const FlutterSecureStorage(
            // Android: AES-GCM with RSA-OAEP key wrapping, held in the
            // KeyStore. migrateWithAlgorithmChange keeps an existing value
            // readable if the cipher choice ever changes between releases.
            aOptions: AndroidOptions(
              keyCipherAlgorithm:
                  KeyCipherAlgorithm.RSA_ECB_OAEPwithSHA_256andMGF1Padding,
              storageCipherAlgorithm: StorageCipherAlgorithm.AES_GCM_NoPadding,
              migrateOnAlgorithmChange: true,
            ),
            iOptions: IOSOptions(
              // The key is only ever read while the app is in the foreground,
              // and it does not travel to another device through a backup.
              accessibility: KeychainAccessibility.first_unlock_this_device,
            ),
          );

  final FlutterSecureStorage _storage;

  static const _key = 'notes_api_key';

  /// Returns the stored key, or null when none has been saved yet.
  Future<String?> read() async {
    try {
      return await _storage.read(key: _key);
    } catch (error) {
      // A keystore can fail on a locked or corrupt profile. That must not stop
      // the app from starting: it falls back to the login screen.
      debugPrint('Failed to read the stored API key: $error');
      return null;
    }
  }

  /// Stores the key, replacing any previous one.
  Future<void> write(String apiKey) async {
    await _storage.write(key: _key, value: apiKey.trim());
  }

  /// Removes the key, which is what a sign-out does.
  Future<void> clear() async {
    try {
      await _storage.delete(key: _key);
    } catch (error) {
      debugPrint('Failed to delete the stored API key: $error');
    }
  }
}
