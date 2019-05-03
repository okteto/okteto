import mixpanel from 'mixpanel-browser';
import ga from 'universal-ga';

const GA_TOKEN = 'UA-120828135-3';
const MIXPANEL_TOKEN = '92fe782cdffa212d8f03861fbf1ea301';

export default {
  init: (user) => {
    ga.initialize(GA_TOKEN);
    mixpanel.init(MIXPANEL_TOKEN);
    mixpanel.identify(user.email);
    mixpanel.people.set({
      '$name': user.name,
      '$email': user.email,
      'oktetoId': user.id,
      'githubId': user.githubID
    })
  },

  track: (eventName, properties = {}) => {
    mixpanel.track(eventName, properties);
  },

  set: (property, value) => {
    const name = property.trim();
    if (name === '') return;
    const obj = {};
    obj[name] = value;
    mixpanel.people.set(obj);
  },

  increment: (property, value = 1) => {
    mixpanel.people.increment(property, value);
  }
};