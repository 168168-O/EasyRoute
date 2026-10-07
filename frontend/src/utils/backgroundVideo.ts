import { ref } from 'vue'

/** Object URL used when the desktop file server is not available (browser preview). */
export const previewVideoURL = ref('')

/** Bumped whenever the background file is replaced, even if the path stays the same. */
export const backgroundVideoNonce = ref(0)

export const bumpBackgroundVideo = () => {
  backgroundVideoNonce.value += 1
}
