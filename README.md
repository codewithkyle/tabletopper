<p align="center">
  <img width="300" alt="tabletopper logo" src="https://user-images.githubusercontent.com/15202776/225735510-2bfe1b47-9937-46a0-bf69-ddc915e0a0f0.png">
</p>

<p align="center">Jump into a D&amp;D tabletop session with your friends, hassle free. <a href="https://tabletopper.app/">Tabletopper.app</a></p>


## Asset URLs

`/assets/images/{id}`, `/assets/images/{id}/preview` and the map tiles under `/assets/maps/{id}/tiles/` need a signed-in session but do not check who owns the asset. That is deliberate: players in a room load the GM's maps, tokens and monster images without being granted each one. The asset's ULID is the capability, so anyone signed in who holds the URL can load the image.
