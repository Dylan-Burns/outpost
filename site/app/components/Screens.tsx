import Image, { type StaticImageData } from "next/image";
import downloads from "../shots/downloads.png";
import downloadsPhone from "../shots/downloads-phone.png";
import subjects from "../shots/subjects.png";
import subjectsPhone from "../shots/subjects-phone.png";
import welcome from "../shots/welcome.png";
import welcomePhone from "../shots/welcome-phone.png";

// The setup pages and the Downloads page as they really look: these are
// photographs of release 1.2.0 with real downloads running, not drawings.
// A wide shot is shown on wide screens and a phone shot on phones, since the
// pages themselves are used from both.

function Screen({
  desk,
  phone,
  alt,
  path,
  crop,
  children,
}: {
  desk: StaticImageData;
  phone: StaticImageData;
  alt: string;
  path: string;
  crop?: boolean;
  children: React.ReactNode;
}) {
  return (
    <figure className={`screen${crop ? " crop" : ""}`}>
      <div className="screen-frame">
        <div className="screen-bar" aria-hidden="true">
          <i />
          <i />
          <i />
          <span>http://192.168.1.40:8080{path}</span>
        </div>
        <Image className="desk" src={desk} alt={alt} sizes="(max-width: 700px) 1px, (max-width: 1100px) 94vw, 1250px" />
        <Image className="phone" src={phone} alt={alt} sizes="(max-width: 700px) 94vw, 1px" />
      </div>
      <figcaption>{children}</figcaption>
    </figure>
  );
}

export default function Screens() {
  return (
    <div className="screens">
      <Screen
        desk={downloads}
        phone={downloadsPhone}
        path="/downloads"
        alt="The Downloads page: 3 of 10 ready, 56 MB of 1.2 GB downloaded, 5.8 MB per second, 4 minutes left, then a row for each download with its state, percentage and a cancel button."
      >
        <strong>Watch it arrive</strong>
        Every download shows what it is doing, how far it has got, how fast it is going and how long is left, updated every second. Close the
        page whenever you like; it carries on.
      </Screen>
      <div className="screen-pair">
        <Screen
          crop
          desk={welcome}
          phone={welcomePhone}
          path="/"
          alt="The welcome page: the setup code is already filled in, with two boxes to choose and repeat a password and a Start setup button."
        >
          <strong>No code to copy</strong>
          On a Mac the setup page opens by itself with the code filled in. You choose a password and you are the owner.
        </Screen>
        <Screen
          crop
          desk={subjects}
          phone={subjectsPhone}
          path="/"
          alt="The Choose your subjects page: each subject, such as Medicine and first aid or Repair and DIY, has four buttons: None, Essential, Standard, Comprehensive, each with its size."
        >
          <strong>Tick what you want</strong>
          Each subject comes in three sizes with the space it takes written on it. A bar at the top shows what fits on your drive.
        </Screen>
      </div>
    </div>
  );
}
